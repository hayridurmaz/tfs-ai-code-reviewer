package publisher

import (
	"context"
	"fmt"
	"strings"

	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/llm"
	"github.com/sirupsen/logrus"
)

type Publisher struct {
	adoClient *ado.Client
}

func NewPublisher(adoClient *ado.Client) *Publisher {
	return &Publisher{adoClient: adoClient}
}

func (p *Publisher) PublishReview(ctx context.Context, repoID string, prID int, iterationID int, result *llm.ReviewResult) error {
	logrus.Debugf("📤 Publishing review for PR #%d (iteration %d): %d summary items, %d comments",
		prID, iterationID, len(result.Summary), len(result.Comments))

	// 1) Summary thread'i oluştur
	summaryText := "### 🤖 AI Code Review Özeti\n\n"
	for _, s := range result.Summary {
		summaryText += fmt.Sprintf("- %s\n", s)
	}

	summaryThread := map[string]interface{}{
		"comments": []map[string]interface{}{
			{
				"parentCommentId": 0,
				"content":         summaryText,
				"commentType":     "text",
			},
		},
		"status": "fixed",
	}

	if err := p.adoClient.PostThread(ctx, repoID, prID, iterationID, summaryThread); err != nil {
		return fmt.Errorf("failed to post summary thread: %w", err)
	}
	logrus.Debugf("📝 Summary thread posted for PR #%d", prID)

	// 2) Spesifik yorumları (line-level) yayınla
	for _, comment := range result.Comments {
		// Normalize path to ensure consistency
		normalizedPath := normalizePath(comment.Path)

		text := fmt.Sprintf("**Severity:** %s\n\n%s", strings.ToUpper(comment.Severity), comment.Message)
		if comment.Suggestion != "" {
			text += fmt.Sprintf("\n\n```suggestion\n%s\n```", comment.Suggestion)
		}

		thread := map[string]interface{}{
			"comments": []map[string]interface{}{
				{
					"parentCommentId": 0,
					"content":         text,
					"commentType":     "text",
				},
			},
			"status": "active",
			"threadContext": map[string]interface{}{
				"filePath": normalizedPath,
				"rightFileStart": map[string]interface{}{
					"line":   comment.Line,
					"offset": 1,
				},
				"rightFileEnd": map[string]interface{}{
					"line":   comment.Line,
					"offset": 1,
				},
			},
			// NOT using pullRequestThreadContext - it was causing comments to go to wrong files
			// The changeTrackingId should be the file's change ID, not the iteration ID
			// For simple file-level comments, threadContext alone is sufficient
		}

		if err := p.adoClient.PostThread(ctx, repoID, prID, iterationID, thread); err != nil {
			return fmt.Errorf("failed to post comment thread for '%s' at line %d: %w", normalizedPath, comment.Line, err)
		}
		logrus.Debugf("💬 Comment posted: %s:%d [%s]", normalizedPath, comment.Line, comment.Severity)
	}

	logrus.Infof("📤 Published %d comments to PR #%d", len(result.Comments), prID)
	return nil
}

// normalizePath ensures consistent path formatting for ADO API
// ADO expects paths to start with "/" for thread context
func normalizePath(path string) string {
	// Remove leading "/" if present, then add it back
	// This ensures we always have exactly one leading "/"
	path = strings.TrimPrefix(path, "/")
	return "/" + path
}
