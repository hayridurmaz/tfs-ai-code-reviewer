package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

type ReviewResult struct {
	Summary  []string  `json:"summary"`
	Comments []Comment `json:"comments"`
}

type Comment struct {
	Path       string  `json:"path"`
	Line       int     `json:"line"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Suggestion string  `json:"suggestion"`
	Confidence float64 `json:"confidence"`
}

type Client struct {
	client     *openai.Client
	model      string
	maxRetries int
}

func NewClient(baseURL, apiKey, model string, maxRetries int) *Client {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	return &Client{
		client:     openai.NewClientWithConfig(config),
		model:      model,
		maxRetries: maxRetries,
	}
}

func (c *Client) ReviewCode(ctx context.Context, systemPrompt, userPrompt string) (*ReviewResult, error) {
	var resp openai.ChatCompletionResponse
	var err error

	for i := 0; i <= c.maxRetries; i++ {
		resp, err = c.client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model: c.model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: systemPrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: userPrompt,
					},
				},
				ResponseFormat: &openai.ChatCompletionResponseFormat{
					Type: openai.ChatCompletionResponseFormatTypeJSONObject,
				},
			},
		)

		if err == nil {
			break
		}

		if i < c.maxRetries {
			backoff := time.Duration(1<<uint(i)) * time.Second
			logrus.Warnf("LLM request failed (attempt %d/%d): %v. Retrying in %v...", i+1, c.maxRetries+1, err, backoff)
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("LLM request failed after %d retries: %w", c.maxRetries, err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := resp.Choices[0].Message.Content

	// Some models might wrap JSON in markdown blocks even with json_object format
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w\nContent: %s", err, content)
	}

	return &result, nil
}
