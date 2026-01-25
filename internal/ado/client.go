package ado

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
	project string
	pat     string
	http    *http.Client
}

func NewClient(baseURL, project, pat string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		project: project,
		pat:     pat,
		http:    &http.Client{},
	}
}

func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.pat))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

type Repository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RepositoriesResponse struct {
	Value []Repository `json:"value"`
}

func (c *Client) GetRepositories() ([]Repository, error) {
	url := fmt.Sprintf("%s/%s/_api/_versioncontrol/repositories", c.baseURL, c.project)
	// Note: The above is for TFS on-prem sometimes. Let's use the standard REST API
	url = fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=6.0", c.baseURL, c.project)

	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repositories: status %d", resp.StatusCode)
	}

	var res RepositoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
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

func (c *Client) GetActivePullRequests(repoID string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=active&api-version=6.0", c.baseURL, c.project, repoID)

	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res PullRequestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Value, nil
}

type Iteration struct {
	ID int `json:"id"`
}

type IterationsResponse struct {
	Value []Iteration `json:"value"`
}

func (c *Client) GetIterations(repoID string, prID int) ([]Iteration, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/iterations?api-version=6.0", c.baseURL, c.project, repoID, prID)

	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res IterationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
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

func (c *Client) GetIterationChanges(repoID string, prID, iterationID int, compareToIterationID *int) ([]Change, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/iterations/%d/changes?api-version=6.0", c.baseURL, c.project, repoID, prID, iterationID)
	if compareToIterationID != nil {
		url += fmt.Sprintf("&compareTo=%d", *compareToIterationID)
	}

	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get iteration changes: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read iteration changes body: %w", err)
	}

	var res ChangesResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse iteration changes JSON: %w, raw body: %s", err, string(body))
	}

	return res.ChangeEntries, nil
}

func (c *Client) GetFileContent(url string) (string, error) {
	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get file content: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *Client) GetBlobContent(repoID, objectID string) (string, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/blobs/%s?api-version=6.0", c.baseURL, c.project, repoID, objectID)

	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get blob content: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *Client) PostThread(repoID string, prID int, iterationID int, threadData interface{}) error {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/threads?api-version=6.0", c.baseURL, c.project, repoID, prID)

	body, err := json.Marshal(threadData)
	if err != nil {
		return err
	}

	req, err := c.newRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post thread: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
