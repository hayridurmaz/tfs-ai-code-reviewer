package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/config"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/llm"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/publisher"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/reviewer"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/state"
	"github.com/sirupsen/logrus"
)

func main() {
	// 1) Load config
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cfg.Bot.LogPath), 0755); err != nil {
		logrus.Fatalf("Failed to create log directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Bot.DBPath), 0755); err != nil {
		logrus.Fatalf("Failed to create DB directory: %v", err)
	}

	// Setup logging
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logFile, err := os.OpenFile(cfg.Bot.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatalf("Failed to open log file: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	logrus.SetOutput(mw)

	logrus.Infof("🚀 AI PR Reviewer Bot (Go) starting...")
	logrus.Infof("ADO: %s, Project: %s, Interval: %ds", cfg.ADO.BaseURL, cfg.ADO.ProjectName, cfg.Bot.PollIntervalSec)

	// 2) Init services
	store, err := state.NewStore(cfg.Bot.DBPath)
	if err != nil {
		logrus.Fatalf("Failed to init state store: %v", err)
	}
	defer store.Close()

	adoClient := ado.NewClient(cfg.ADO.BaseURL, cfg.ADO.ProjectName, cfg.ADO.PAT)
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.MaxRetries)
	rev := reviewer.NewReviewer(cfg, adoClient, llmClient)
	pub := publisher.NewPublisher(adoClient)

	// 3) Polling loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Info("Gracefully shutting down...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			pollOnce(ctx, cfg, store, adoClient, rev, pub)
			logrus.Infof("✅ Polling completed. Waiting %ds...", cfg.Bot.PollIntervalSec)

			select {
			case <-time.After(time.Duration(cfg.Bot.PollIntervalSec) * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

func pollOnce(ctx context.Context, cfg *config.Config, store *state.Store, adoClient *ado.Client, rev *reviewer.Reviewer, pub *publisher.Publisher) {
	logrus.Info("🔄 Polling started...")

	repos, err := adoClient.GetRepositories()
	if err != nil {
		logrus.Errorf("Failed to get repositories: %v", err)
		return
	}

	// Filter repos if specified
	if len(cfg.ADO.Repos) > 0 {
		filtered := make([]ado.Repository, 0)
		for _, r := range repos {
			for _, allowed := range cfg.ADO.Repos {
				if r.Name == allowed {
					filtered = append(filtered, r)
					break
				}
			}
		}
		repos = filtered
	}

	for _, repo := range repos {
		processRepo(ctx, cfg, repo, store, adoClient, rev, pub)
	}
}

func processRepo(ctx context.Context, cfg *config.Config, repo ado.Repository, store *state.Store, adoClient *ado.Client, rev *reviewer.Reviewer, pub *publisher.Publisher) {
	logrus.Infof("📂 Repo: %s (%s)", repo.Name, repo.ID)

	prs, err := adoClient.GetActivePullRequests(repo.ID)
	if err != nil {
		logrus.Errorf("Failed to get active PRs for %s: %v", repo.Name, err)
		return
	}

	// Filter by branch
	if len(cfg.ADO.TargetBranches) > 0 {
		filtered := make([]ado.PullRequest, 0)
		for _, pr := range prs {
			matched := false
			for _, b := range cfg.ADO.TargetBranches {
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
	if len(cfg.ADO.IgnorePRIDs) > 0 {
		filtered := make([]ado.PullRequest, 0)
		for _, pr := range prs {
			ignored := false
			for _, id := range cfg.ADO.IgnorePRIDs {
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

	logrus.Infof("  └─ %s: %d active PRs found", repo.Name, len(prs))

	for _, pr := range prs {
		processPR(ctx, cfg, repo, pr, store, adoClient, rev, pub)
	}
}

func processPR(ctx context.Context, cfg *config.Config, repo ado.Repository, pr ado.PullRequest, store *state.Store, adoClient *ado.Client, rev *reviewer.Reviewer, pub *publisher.Publisher) {
	iterations, err := adoClient.GetIterations(repo.ID, pr.PullRequestId)
	if err != nil {
		logrus.Errorf("Failed to get iterations for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if len(iterations) == 0 {
		return
	}

	latestIteration := iterations[len(iterations)-1]

	reviewed, err := store.IsIterationReviewed(repo.ID, pr.PullRequestId, latestIteration.ID)
	if err != nil {
		logrus.Errorf("Database error checking PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if reviewed {
		return
	}

	logrus.Infof("🔍 PR #%d - iteration %d is being reviewed...", pr.PullRequestId, latestIteration.ID)

	var compareTo *int
	if len(iterations) > 1 {
		prev := iterations[len(iterations)-2].ID
		compareTo = &prev
		logrus.Infof("  ℹ️ Incremental Review: Iteration %d -> %d", *compareTo, latestIteration.ID)
	}

	changes, err := adoClient.GetIterationChanges(repo.ID, pr.PullRequestId, latestIteration.ID, compareTo)
	if err != nil {
		logrus.Errorf("Failed to get changes for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	result, err := rev.ReviewPR(ctx, repo.ID, pr, changes)
	if err != nil {
		logrus.Errorf("Review failed for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	if cfg.Bot.DryRun {
		logrus.Infof("--- DRY RUN: Review for PR #%d ---", pr.PullRequestId)
		logrus.Infof("Summary: %v", result.Summary)
		for _, c := range result.Comments {
			logrus.Infof("[%s:%d] %s: %s", c.Path, c.Line, c.Severity, c.Message)
		}
		return
	}

	if err := pub.PublishReview(repo.ID, pr.PullRequestId, latestIteration.ID, result); err != nil {
		logrus.Errorf("Failed to publish review for PR #%d: %v", pr.PullRequestId, err)
		return
	}

	resJSON, _ := json.Marshal(result)
	if err := store.MarkIterationReviewed(repo.ID, pr.PullRequestId, latestIteration.ID, string(resJSON)); err != nil {
		logrus.Errorf("Failed to mark PR #%d as reviewed in database: %v", pr.PullRequestId, err)
	}

	logrus.Infof("✅ PR #%d reviewed and comments posted", pr.PullRequestId)
}
