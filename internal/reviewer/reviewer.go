package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

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
		content, err = r.adoClient.GetBlobContent(ctx, repoID, change.Item.ObjectId)
	} else if change.Item.URL != "" {
		content, err = r.adoClient.GetFileContent(ctx, change.Item.URL)
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
		if change.Item.OriginalObjectId != "" {
			originalContent, err = r.adoClient.GetBlobContent(ctx, repoID, change.Item.OriginalObjectId)
			if err != nil {
				logrus.Warnf("Failed to get original content (via blob) for %s: %v", change.Item.Path, err)
				originalContent = ""
			}
		} else if change.Item.OriginalURL != "" {
			originalContent, err = r.adoClient.GetFileContent(ctx, change.Item.OriginalURL)
			if err != nil {
				logrus.Warnf("Failed to get original content (via URL) for %s: %v", change.Item.Path, err)
				originalContent = ""
			}
		}
	}

	edits := myers.ComputeEdits(span.URIFromPath(change.Item.Path), originalContent, content)
	diff := fmt.Sprint(gotextdiff.ToUnified(change.Item.Path, change.Item.Path, originalContent, edits))

	return &FileDiff{
		Path:       change.Item.Path,
		Diff:       diff,
		Content:    content,
		ChangeType: change.ChangeType,
	}, nil
}

// validateAndFilterComments ensures all comments reference valid files from the PR
func (r *Reviewer) validateAndFilterComments(comments []llm.Comment, fileDiffs []FileDiff) []llm.Comment {
	// Build a map of valid file paths for quick lookup
	// Normalize paths by removing leading "/" to handle inconsistent formatting
	validPaths := make(map[string]bool)
	for _, fd := range fileDiffs {
		normalizedPath := strings.TrimPrefix(fd.Path, "/")
		validPaths[normalizedPath] = true
		// Also add the original path in case it doesn't have a leading "/"
		validPaths[fd.Path] = true
	}

	validComments := make([]llm.Comment, 0, len(comments))
	for _, c := range comments {
		// Normalize the comment path for comparison
		normalizedCommentPath := strings.TrimPrefix(c.Path, "/")

		// Check if path is valid (try both normalized and original)
		if !validPaths[c.Path] && !validPaths[normalizedCommentPath] {
			logrus.Warnf("⚠️  Invalid comment path from LLM: '%s' (not in changed files). Skipping comment.", c.Path)
			continue
		}

		// Check if line number is reasonable
		if c.Line <= 0 {
			logrus.Warnf("⚠️  Invalid line number %d for file '%s'. Skipping comment.", c.Line, c.Path)
			continue
		}

		validComments = append(validComments, c)
	}

	if len(comments) > len(validComments) {
		logrus.Warnf("Filtered out %d invalid comments (wrong paths or line numbers)", len(comments)-len(validComments))
	}

	return validComments
}

func (r *Reviewer) ReviewPR(ctx context.Context, repoID string, pr ado.PullRequest, changes []ado.Change) (*llm.ReviewResult, error) {
	var fileDiffs []FileDiff
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent file processing within a single PR review
	fileSemaphore := make(chan struct{}, 10)

	for _, change := range changes {
		wg.Add(1)
		go func(c ado.Change) {
			defer wg.Done()

			fileSemaphore <- struct{}{}
			defer func() { <-fileSemaphore }()

			fd, err := r.prepareFileContext(ctx, repoID, c)
			if err != nil {
				logrus.Errorf("Error preparing context for %s: %v", c.Item.Path, err)
				return
			}
			if fd != nil {
				mu.Lock()
				fileDiffs = append(fileDiffs, *fd)
				mu.Unlock()
			}
		}(change)
	}
	wg.Wait()

	if len(fileDiffs) == 0 {
		return &llm.ReviewResult{
			Summary:  []string{"İncelenecek uygun dosya bulunamadı (ignore rules veya size limit)."},
			Comments: []llm.Comment{},
		}, nil
	}

	// Determine if batching is needed
	batchSize := r.cfg.Bot.MaxFilesPerBatch
	if batchSize <= 0 || len(fileDiffs) <= batchSize {
		// No batching needed - review all files in one request
		logrus.Infof("Reviewing %d files in single request (no batching)", len(fileDiffs))
		return r.reviewBatch(ctx, pr, fileDiffs)
	}

	// Use batching for large PRs
	logrus.Infof("Reviewing %d files in batches of %d", len(fileDiffs), batchSize)
	return r.reviewInBatches(ctx, pr, fileDiffs, batchSize)
}

// reviewBatch reviews a single batch of files
func (r *Reviewer) reviewBatch(ctx context.Context, pr ado.PullRequest, fileDiffs []FileDiff) (*llm.ReviewResult, error) {
	userPrompt := BuildUserPrompt(pr.Title, pr.Description, fileDiffs)

	// 1. AŞAMA: Draft Review
	result, err := r.llmClient.ReviewCode(ctx, SystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 2. AŞAMA: Self-Correction (Opsiyonel)
	if r.cfg.Bot.EnableSelfCorrection {
		logrus.Infof("🧠 Applying Self-Correction (2-Stage Verification)...")

		// Taslak sonucu JSON'a çevir
		draftJSON, _ := json.MarshalIndent(result, "", "  ") // Error ignored, safe for simple struct

		// Yeni user prompt: Kod Context + Draft JSON
		correctionUserPrompt := fmt.Sprintf("ORİJİNAL KOD CONTEXT:\n%s\n\nTASLAK İNCELEME RAPORU (DÜZELTİLECEK):\n```json\n%s\n```\n\nLütfen yukarıdaki raporu denetle, gereksizleri sil ve formatı düzelt.", userPrompt, string(draftJSON))

		// LLM'e tekrar sor
		correctedResult, err := r.llmClient.ReviewCode(ctx, SelfCorrectionPrompt, correctionUserPrompt)
		if err == nil {
			// Eğer başarılıysa, sonucu güncelle
			countDiff := len(result.Comments) - len(correctedResult.Comments)
			logrus.Infof("✅ Self-Correction complete. Filtered %d comments. Final count: %d", countDiff, len(correctedResult.Comments))
			result = correctedResult
		} else {
			logrus.Warnf("⚠️ Self-Correction failed: %v. Using original draft.", err)
		}
	}

	// Clean suggestions (Post-process cleanup is still useful)
	for i := range result.Comments {
		result.Comments[i].Suggestion = r.cleanSuggestion(result.Comments[i].Suggestion)
	}

	// Validate comments against actual changed files
	result.Comments = r.validateAndFilterComments(result.Comments, fileDiffs)

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

// cleanSuggestion removes markdown formatting and extra whitespace from valid suggestions
func (r *Reviewer) cleanSuggestion(suggestion string) string {
	if suggestion == "" {
		return ""
	}

	// Remove markdown code blocks
	s := strings.TrimSpace(suggestion)

	// Handle opening ```
	if strings.HasPrefix(s, "```") {
		// Find end of first line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		} else {
			// Only has opening ticks and maybe language name
			s = strings.TrimPrefix(s, "```")
		}
	}

	// Handle closing ```
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
}

// reviewInBatches splits files into batches and reviews each separately
func (r *Reviewer) reviewInBatches(ctx context.Context, pr ado.PullRequest, fileDiffs []FileDiff, batchSize int) (*llm.ReviewResult, error) {
	var allComments []llm.Comment
	var allSummaries []string
	var mu sync.Mutex

	numBatches := (len(fileDiffs) + batchSize - 1) / batchSize

	for i := 0; i < len(fileDiffs); i += batchSize {
		end := i + batchSize
		if end > len(fileDiffs) {
			end = len(fileDiffs)
		}

		batch := fileDiffs[i:end]
		batchNum := i/batchSize + 1

		logrus.Infof("📦 Batch %d/%d: Reviewing %d files", batchNum, numBatches, len(batch))

		result, err := r.reviewBatch(ctx, pr, batch)
		if err != nil {
			logrus.Errorf("❌ Batch %d/%d failed: %v (continuing with other batches)", batchNum, numBatches, err)
			continue // Continue with other batches even if one fails
		}

		mu.Lock()
		allComments = append(allComments, result.Comments...)
		allSummaries = append(allSummaries, result.Summary...)
		mu.Unlock()

		logrus.Infof("✅ Batch %d/%d complete: %d comments", batchNum, numBatches, len(result.Comments))
	}

	// Deduplicate summaries (simple approach: keep unique ones)
	uniqueSummaries := deduplicateSummaries(allSummaries)

	logrus.Infof("🎯 Batching complete: %d total comments from %d batches", len(allComments), numBatches)

	return &llm.ReviewResult{
		Summary:  uniqueSummaries,
		Comments: allComments,
	}, nil
}

// deduplicateSummaries removes duplicate summary entries
func deduplicateSummaries(summaries []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0)

	for _, s := range summaries {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}

	return unique
}

func sortedKeys(m map[string][]llm.Comment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
