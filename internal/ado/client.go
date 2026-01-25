package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	project string
	pat     string
	http    *http.Client
}

func NewClient(baseURL, project, pat string, timeoutSec int) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		project: project,
		pat:     pat,
		http: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.pat))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (c *Client) doRequest(ctx context.Context, url string, result interface{}) error {
	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

type Repository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RepositoriesResponse struct {
	Value []Repository `json:"value"`
}

func (c *Client) GetRepositories(ctx context.Context) ([]Repository, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=6.0", c.baseURL, c.project)
	var res RepositoriesResponse
	if err := c.doRequest(ctx, url, &res); err != nil {
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}
	return res.Value, nil
}

type PullRequest struct {
	PullRequestId int    `json:"pullRequestId"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	TargetRefName string `json:"targetRefName"`
}

type PullRequestsResponse struct {
	Value []PullRequest `json:"value"`
}

func (c *Client) GetActivePullRequests(ctx context.Context, repoID string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=active&api-version=6.0", c.baseURL, c.project, repoID)
	var res PullRequestsResponse
	if err := c.doRequest(ctx, url, &res); err != nil {
		return nil, fmt.Errorf("failed to get active PRs: %w", err)
	}
	return res.Value, nil
}

type Iteration struct {
	ID int `json:"id"`
}

type IterationsResponse struct {
	Value []Iteration `json:"value"`
}

func (c *Client) GetIterations(ctx context.Context, repoID string, prID int) ([]Iteration, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/iterations?api-version=6.0", c.baseURL, c.project, repoID, prID)
	var res IterationsResponse
	if err := c.doRequest(ctx, url, &res); err != nil {
		return nil, fmt.Errorf("failed to get iterations: %w", err)
	}
	return res.Value, nil
}

type Change struct {
	ID         int    `json:"changeId"`
	ChangeType string `json:"changeType"`
	Item       struct {
		Path        string `json:"path"`
		URL         string `json:"url"`
		OriginalURL string `json:"originalUrl"`
		ObjectId    string `json:"objectId"`
	} `json:"item"`
}

type ChangesResponse struct {
	ChangeEntries []Change `json:"changeEntries"`
}

func (c *Client) GetIterationChanges(ctx context.Context, repoID string, prID, iterationID int, compareToIterationID *int) ([]Change, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/iterations/%d/changes?api-version=6.0", c.baseURL, c.project, repoID, prID, iterationID)
	if compareToIterationID != nil {
		url += fmt.Sprintf("&compareTo=%d", *compareToIterationID)
	}

	var res ChangesResponse
	if err := c.doRequest(ctx, url, &res); err != nil {
		return nil, fmt.Errorf("failed to get iteration changes: %w", err)
	}
	return res.ChangeEntries, nil
}

func (c *Client) GetFileContent(ctx context.Context, url string) (string, error) {
	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get file content: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	return string(body), nil
}

func (c *Client) GetBlobContent(ctx context.Context, repoID, objectID string) (string, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/blobs/%s?api-version=6.0", c.baseURL, c.project, repoID, objectID)

	req, err := c.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	return string(body), nil
}

func (c *Client) PostThread(ctx context.Context, repoID string, prID int, iterationID int, threadData interface{}) error {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/threads?api-version=6.0", c.baseURL, c.project, repoID, prID)

	body, err := json.Marshal(threadData)
	if err != nil {
		return fmt.Errorf("failed to marshal thread: %w", err)
	}

	req, err := c.newRequest(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post thread: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
