package reviewer

import (
	"context"
	"fmt"
	"sort"

	"github.com/gobwas/glob"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/config"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/llm"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sirupsen/logrus"
)

type Reviewer struct {
	cfg         *config.Config
	adoClient   *ado.Client
	llmClient   *llm.Client
	ignoreGlobs []glob.Glob
}

func NewReviewer(cfg *config.Config, adoClient *ado.Client, llmClient *llm.Client) *Reviewer {
	globs := make([]glob.Glob, 0, len(cfg.IgnorePatterns))
	for _, p := range cfg.IgnorePatterns {
		g, err := glob.Compile(p)
		if err == nil {
			globs = append(globs, g)
		} else {
			logrus.Warnf("Failed to compile ignore pattern: %s, error: %v", p, err)
		}
	}

	return &Reviewer{
		cfg:         cfg,
		adoClient:   adoClient,
		llmClient:   llmClient,
		ignoreGlobs: globs,
	}
}

func (r *Reviewer) shouldIgnoreFile(path string) bool {
	for _, g := range r.ignoreGlobs {
		if g.Match(path) {
			return true
		}
	}
	return false
}

func (r *Reviewer) prepareFileContext(ctx context.Context, repoID string, change ado.Change) (*FileDiff, error) {
	if change.Item.Path == "" || change.ChangeType == "delete" {
		return nil, nil
	}

	if r.shouldIgnoreFile(change.Item.Path) {
		return nil, nil
	}

	var content string
	var err error

	if change.Item.ObjectId != "" {
		content, err = r.adoClient.GetBlobContent(repoID, change.Item.ObjectId)
	} else if change.Item.URL != "" {
		content, err = r.adoClient.GetFileContent(change.Item.URL)
	} else {
		return nil, fmt.Errorf("no objectId or URL for %s", change.Item.Path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get content for %s: %w", change.Item.Path, err)
	}

	if int64(len(content)) > r.cfg.Bot.MaxFileSizeBytes {
		logrus.Warnf("File too large, skipping: %s", change.Item.Path)
		return nil, nil
	}

	var originalContent string
	if change.ChangeType == "edit" {
		if change.Item.OriginalURL != "" {
			originalContent, err = r.adoClient.GetFileContent(change.Item.OriginalURL)
			if err != nil {
				logrus.Warnf("Failed to get original content for %s: %v", change.Item.Path, err)
				originalContent = ""
			}
		}
		// Note: If OriginalURL is empty, we might need a way to find the base blob.
		// For now, we fall back to empty string for the original content.
	}

	edits := myers.ComputeEdits(span.URIFromPath(change.Item.Path), originalContent, content)
	diff := fmt.Sprint(gotextdiff.ToUnified(change.Item.Path, change.Item.Path, originalContent, edits))

	return &FileDiff{
		Path:       change.Item.Path,
		Diff:       diff,
		ChangeType: change.ChangeType,
	}, nil
}

func (r *Reviewer) ReviewPR(ctx context.Context, repoID string, pr ado.PullRequest, changes []ado.Change) (*llm.ReviewResult, error) {
	var fileDiffs []FileDiff
	for _, change := range changes {
		fd, err := r.prepareFileContext(ctx, repoID, change)
		if err != nil {
			logrus.Errorf("Error preparing context for %s: %v", change.Item.Path, err)
			continue
		}
		if fd != nil {
			fileDiffs = append(fileDiffs, *fd)
		}
	}

	if len(fileDiffs) == 0 {
		return &llm.ReviewResult{
			Summary:  []string{"İncelenecek uygun dosya bulunamadı (ignore rules veya size limit)."},
			Comments: []llm.Comment{},
		}, nil
	}

	userPrompt := BuildUserPrompt(pr.Title, pr.Description, fileDiffs)

	logrus.Infof("Sending to LLM... (%d files)", len(fileDiffs))
	result, err := r.llmClient.ReviewCode(ctx, SystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Filter comments by confidence
	filteredComments := make([]llm.Comment, 0)
	for _, c := range result.Comments {
		if c.Confidence >= r.cfg.Bot.MinConfidence {
			filteredComments = append(filteredComments, c)
		}
	}

	// Group by file and limit comments per file
	grouped := make(map[string][]llm.Comment)
	for _, c := range filteredComments {
		grouped[c.Path] = append(grouped[c.Path], c)
	}

	finalComments := make([]llm.Comment, 0)
	for _, path := range sortedKeys(grouped) {
		comments := grouped[path]
		if len(comments) > r.cfg.Bot.MaxCommentsPerFile {
			comments = comments[:r.cfg.Bot.MaxCommentsPerFile]
		}
		finalComments = append(finalComments, comments...)
	}

	result.Comments = finalComments
	return result, nil
}

func sortedKeys(m map[string][]llm.Comment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
