package orchestrator

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/config"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/publisher"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/reviewer"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/state"
	"github.com/sirupsen/logrus"
)

type Orchestrator struct {
	cfg       *config.Config
	store     *state.Store
	adoClient *ado.Client
	rev       *reviewer.Reviewer
	pub       *publisher.Publisher
	tracker   *Tracker
}

func NewOrchestrator(cfg *config.Config, store *state.Store, adoClient *ado.Client, rev *reviewer.Reviewer, pub *publisher.Publisher) *Orchestrator {
	return &Orchestrator{
		cfg:       cfg,
		store:     store,
		adoClient: adoClient,
		rev:       rev,
		pub:       pub,
		tracker:   NewTracker(),
	}
}

func (o *Orchestrator) PollOnce(ctx context.Context) {
	logrus.Debug("🔄 Polling started...")

	repos, err := o.adoClient.GetRepositories(ctx)
	if err != nil {
		logrus.Errorf("Failed to get repositories: %v", err)
		return
	}

	// Filter repos if specified
	if len(o.cfg.ADO.Repos) > 0 {
		filtered := make([]ado.Repository, 0)
		for _, r := range repos {
			for _, allowed := range o.cfg.ADO.Repos {
				if r.Name == allowed {
					filtered = append(filtered, r)
					break
				}
			}
		}
		repos = filtered
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, o.cfg.Bot.MaxConcurrentPRs)

	for _, repo := range repos {
		wg.Add(1)
		go func(r ado.Repository) {
			defer wg.Done()
			o.processRepo(ctx, r, semaphore)
		}(repo)
	}
	wg.Wait()
}

func (o *Orchestrator) processRepo(ctx context.Context, repo ado.Repository, semaphore chan struct{}) {
	logrus.Debugf("📂 Repo: %s (%s)", repo.Name, repo.ID)

	prs, err := o.adoClient.GetActivePullRequests(ctx, repo.ID)
	if err != nil {
		logrus.Errorf("Failed to get active PRs for %s: %v", repo.Name, err)
		return
	}

	// Filter by branch
	if len(o.cfg.ADO.TargetBranches) > 0 {
		filtered := make([]ado.PullRequest, 0)
		for _, pr := range prs {
			matched := false
			for _, b := range o.cfg.ADO.TargetBranches {
				if pr.TargetRefName == b || pr.TargetRefName == "refs/heads/"+b {
					matched = true
					break
				}
			}
			if matched {
				filtered = append(filtered, pr)
			}
		}
		prs = filtered
	}

	// Filter by ignore PR IDs
	if len(o.cfg.ADO.IgnorePRIDs) > 0 {
		filtered := make([]ado.PullRequest, 0)
		for _, pr := range prs {
			ignored := false
			for _, id := range o.cfg.ADO.IgnorePRIDs {
				if pr.PullRequestId == id {
					ignored = true
					break
				}
			}
			if !ignored {
				filtered = append(filtered, pr)
			}
		}
		prs = filtered
	}
	if len(prs) > 0 {
		logrus.Debugf("  └─ %s: %d active PRs found", repo.Name, len(prs))
	}

	for _, pr := range prs {
		semaphore <- struct{}{} // Acquire semaphore
		go func(p ado.PullRequest) {
			defer func() { <-semaphore }() // Release semaphore
			o.processPR(ctx, repo, p)
		}(pr)
	}
}

func (o *Orchestrator) processPR(ctx context.Context, repo ado.Repository, pr ado.PullRequest) {
	iterations, err := o.adoClient.GetIterations(ctx, repo.ID, pr.PullRequestId)
	if err != nil {
		logrus.Errorf("Failed to get iterations for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if len(iterations) == 0 {
		return
	}

	latestIteration := iterations[len(iterations)-1]

	// Check if already being reviewed by another goroutine
	if !o.tracker.Lock(repo.ID, pr.PullRequestId, latestIteration.ID) {
		return
	}
	defer o.tracker.Unlock(repo.ID, pr.PullRequestId, latestIteration.ID)

	reviewed, err := o.store.IsIterationReviewed(repo.ID, pr.PullRequestId, latestIteration.ID)
	if err != nil {
		logrus.Errorf("Database error checking PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if reviewed {
		return
	}

	logrus.Infof("🔍 PR #%d - iteration %d is being reviewed...", pr.PullRequestId, latestIteration.ID)

	var compareTo *int
	isFirstReview := len(iterations) == 1
	if len(iterations) > 1 {
		prev := iterations[len(iterations)-2].ID
		compareTo = &prev
		logrus.Debugf("  ℹ️ Incremental Review: Iteration %d -> %d", *compareTo, latestIteration.ID)
	} else {
		logrus.Debugf("  ℹ️ First Review: Iteration %d", latestIteration.ID)
	}

	changes, err := o.adoClient.GetIterationChanges(ctx, repo.ID, pr.PullRequestId, latestIteration.ID, compareTo)
	if err != nil {
		logrus.Errorf("Failed to get changes for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	result, err := o.rev.ReviewPR(ctx, repo.ID, pr, changes, isFirstReview)
	if err != nil {
		logrus.Errorf("Review failed for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if o.cfg.Bot.DryRun {
		logrus.Infof("--- DRY RUN: Review for PR #%d ---", pr.PullRequestId)
		logrus.Infof("Summary: %v", result.Summary)
		for _, c := range result.Comments {
			logrus.Infof("[%s:%d] %s: %s", c.Path, c.Line, c.Severity, c.Message)
		}
		return
	}

	if err := o.pub.PublishReview(ctx, repo.ID, pr.PullRequestId, latestIteration.ID, result); err != nil {
		logrus.Errorf("Failed to publish review for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	resJSON, _ := json.Marshal(result)
	if err := o.store.MarkIterationReviewed(repo.ID, pr.PullRequestId, latestIteration.ID, string(resJSON)); err != nil {
		logrus.Errorf("Failed to mark PR #%d as reviewed in database: %v", pr.PullRequestId, err)
	}

	logrus.Infof("✅ PR #%d reviewed and comments posted", pr.PullRequestId)
}
