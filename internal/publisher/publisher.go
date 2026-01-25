package publisher

import (
	"fmt"
	"strings"

	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/llm"
)

type Publisher struct {
	adoClient *ado.Client
}

func NewPublisher(adoClient *ado.Client) *Publisher {
	return &Publisher{adoClient: adoClient}
}

func (p *Publisher) PublishReview(repoID string, prID int, iterationID int, result *llm.ReviewResult) error {
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

	if err := p.adoClient.PostThread(repoID, prID, iterationID, summaryThread); err != nil {
		return fmt.Errorf("failed to post summary thread: %w", err)
	}

	// 2) Spesifik yorumları (line-level) yayınla
	for _, comment := range result.Comments {
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
				"filePath": comment.Path,
				"rightFileStart": map[string]interface{}{
					"line":   comment.Line,
					"offset": 1,
				},
				"rightFileEnd": map[string]interface{}{
					"line":   comment.Line,
					"offset": 1,
				},
			},
			"pullRequestThreadContext": map[string]interface{}{
				"changeTrackingId": iterationID, // Iteration bazlı thread
				"iterationContext": map[string]interface{}{
					"firstComparingIteration":  iterationID,
					"secondComparingIteration": iterationID,
				},
			},
		}

		if err := p.adoClient.PostThread(repoID, prID, iterationID, thread); err != nil {
			return fmt.Errorf("failed to post comment thread for %s: %w", comment.Path, err)
		}
	}

	return nil
}
