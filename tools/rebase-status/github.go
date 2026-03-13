package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// GitHubClient wraps HTTP calls to the GitHub REST API.
type GitHubClient struct {
	httpClient *http.Client
	token      string
	baseURL    string

	// Simple in-memory cache for directory listings (avoids repeated calls
	// to the same openshift/release path).
	cache   map[string][]byte
	cacheMu sync.Mutex
}

// NewGitHubClient creates a client, resolving the token from GITHUB_TOKEN
// env var or falling back to `gh auth token`.
func NewGitHubClient() (*GitHubClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, fmt.Errorf("no GITHUB_TOKEN and `gh auth token` failed: %w", err)
		}
		token = strings.TrimSpace(string(out))
	}
	if token == "" {
		return nil, fmt.Errorf("could not obtain GitHub token")
	}
	return &GitHubClient{
		httpClient: &http.Client{},
		token:      token,
		baseURL:    "https://api.github.com",
		cache:      make(map[string][]byte),
	}, nil
}

func (c *GitHubClient) get(path string) ([]byte, int, error) {
	url := c.baseURL + path

	c.cacheMu.Lock()
	if cached, ok := c.cache[url]; ok {
		c.cacheMu.Unlock()
		return cached, http.StatusOK, nil
	}
	c.cacheMu.Unlock()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode == http.StatusOK {
		c.cacheMu.Lock()
		c.cache[url] = body
		c.cacheMu.Unlock()
	}

	return body, resp.StatusCode, nil
}

// BranchExists checks if a branch exists in a repo.
func (c *GitHubClient) BranchExists(org, repo, branch string) (bool, error) {
	_, code, err := c.get(fmt.Sprintf("/repos/%s/%s/branches/%s", org, repo, branch))
	if err != nil {
		return false, err
	}
	return code == http.StatusOK, nil
}

// FileContent fetches a file's decoded content from a specific ref.
// Returns nil, nil if the file does not exist (404).
func (c *GitHubClient) FileContent(org, repo, path, ref string) ([]byte, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", org, repo, path)
	if ref != "" {
		apiPath += "?ref=" + ref
	}

	body, code, err := c.get(apiPath)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, nil
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if result.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", result.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(result.Content, "\n", ""),
	)
	if err != nil {
		return nil, fmt.Errorf("decoding base64: %w", err)
	}
	return decoded, nil
}

// DirListing returns the file/directory names in a repo path.
// Returns nil, nil if the path does not exist.
func (c *GitHubClient) DirListing(org, repo, path string) ([]string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", org, repo, path)

	body, code, err := c.get(apiPath)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, nil
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing dir listing: %w", err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// DirExists checks if a directory path exists in a repo (optionally at a ref).
func (c *GitHubClient) DirExists(org, repo, path, ref string) (bool, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", org, repo, path)
	if ref != "" {
		apiPath += "?ref=" + ref
	}
	_, code, err := c.get(apiPath)
	if err != nil {
		return false, err
	}
	return code == http.StatusOK, nil
}

// HeadCommitSHA returns the full SHA of the HEAD commit on a branch.
// Returns "", nil if the branch does not exist.
func (c *GitHubClient) HeadCommitSHA(org, repo, branch string) (string, error) {
	body, code, err := c.get(fmt.Sprintf("/repos/%s/%s/commits/%s", org, repo, branch))
	if err != nil {
		return "", err
	}
	if code == http.StatusNotFound {
		return "", nil
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", code)
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing commit: %w", err)
	}
	return result.SHA, nil
}

// RateLimitRemaining returns the remaining API calls.
func (c *GitHubClient) RateLimitRemaining() (int, error) {
	body, code, err := c.get("/rate_limit")
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK {
		return 0, fmt.Errorf("rate limit check returned %d", code)
	}
	var result struct {
		Resources struct {
			Core struct {
				Remaining int `json:"remaining"`
			} `json:"core"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	return result.Resources.Core.Remaining, nil
}
