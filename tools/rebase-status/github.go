package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
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

// DirListingRef returns the file/directory names in a repo path at a specific ref.
// Returns nil, nil if the path does not exist.
func (c *GitHubClient) DirListingRef(org, repo, path, ref string) ([]string, error) {
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

// CompareCommits returns the commits between base and head (base...head).
// It uses the GitHub compare API. Only the first line of each commit message is returned.
func (c *GitHubClient) CompareCommits(org, repo, base, head string) ([]CommitInfo, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", org, repo, base, head)

	body, code, err := c.get(apiPath)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var result struct {
		Commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing compare: %w", err)
	}

	commits := make([]CommitInfo, len(result.Commits))
	for i, c := range result.Commits {
		msg := c.Commit.Message
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		commits[i] = CommitInfo{SHA: c.SHA, Message: msg}
	}
	return commits, nil
}

// CompareStatus returns the relationship between base and head commits.
// Returns one of: "ahead", "behind", "identical", "diverged".
func (c *GitHubClient) CompareStatus(org, repo, base, head string) (string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", org, repo, base, head)

	body, code, err := c.get(apiPath)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing compare status: %w", err)
	}
	return result.Status, nil
}

// OpenRebasePR searches for an open PR authored by oadp-rebasebot in the
// given repo targeting the specified base branch. Returns nil if none found.
func (c *GitHubClient) OpenRebasePR(org, repo, base string) (*OpenPRInfo, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/pulls?state=open&base=%s&per_page=10", org, repo, base)

	body, code, err := c.get(apiPath)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var prs []struct {
		Number    int       `json:"number"`
		HTMLURL   string    `json:"html_url"`
		CreatedAt time.Time `json:"created_at"`
		User      struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &prs); err != nil {
		return nil, fmt.Errorf("parsing PRs: %w", err)
	}

	for _, pr := range prs {
		login := strings.ToLower(pr.User.Login)
		if login == "oadp-rebasebot" || strings.HasPrefix(login, "oadp-rebasebot-app") {
			return &OpenPRInfo{
				Number:    pr.Number,
				URL:       pr.HTMLURL,
				CreatedAt: pr.CreatedAt,
			}, nil
		}
	}
	return nil, nil
}

// SearchCVEPRs returns open PRs targeting the given base branch that have
// "cve" (case-insensitive) in the title.
func (c *GitHubClient) SearchCVEPRs(org, repo, branch string) ([]CVEPRInfo, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/pulls?state=open&base=%s&per_page=100", org, repo, branch)

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

	var prs []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		HTMLURL   string    `json:"html_url"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(body, &prs); err != nil {
		return nil, fmt.Errorf("parsing PRs: %w", err)
	}

	var result []CVEPRInfo
	for _, pr := range prs {
		if strings.Contains(strings.ToLower(pr.Title), "cve") {
			result = append(result, CVEPRInfo{
				Org:     org,
				Repo:    repo,
				Number:  pr.Number,
				Title:   pr.Title,
				URL:     pr.HTMLURL,
				Created: pr.CreatedAt,
			})
		}
	}
	return result, nil
}

// SearchRebasePRCounts returns the number of rebase PRs opened and merged
// for a given base branch since the specified time. Uses the GitHub Search API
// which returns total_count without requiring pagination.
func (c *GitHubClient) SearchRebasePRCounts(branch string, since time.Time) (opened, merged int, err error) {
	sinceStr := since.Format(time.RFC3339)

	// Count PRs opened since reset
	openedQuery := fmt.Sprintf("is:pr author:oadp-rebasebot base:%s created:>=%s", branch, sinceStr)
	openedCount, err := c.searchIssuesCount(openedQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("searching opened PRs: %w", err)
	}

	// Count PRs merged since reset
	mergedQuery := fmt.Sprintf("is:pr author:oadp-rebasebot base:%s merged:>=%s", branch, sinceStr)
	mergedCount, err := c.searchIssuesCount(mergedQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("searching merged PRs: %w", err)
	}

	return openedCount, mergedCount, nil
}

// CountBranchWorkflowRuns returns the number of auto-rebase-v2 workflow runs
// that targeted the given branch (based on workflow_dispatch inputs) since the
// specified time. It fetches each run's details to inspect the per-branch input.
func (c *GitHubClient) CountBranchWorkflowRuns(owner, repo, branch string, since time.Time) (int, error) {
	inputKey := branchToInputKey(branch)
	if inputKey == "" {
		return 0, fmt.Errorf("no input key mapping for branch %q", branch)
	}

	sinceStr := since.Format(time.RFC3339)
	count := 0
	page := 1

	for {
		apiPath := fmt.Sprintf("/repos/%s/%s/actions/workflows/auto-rebase-v2.yaml/runs?per_page=100&page=%d&created=%s",
			owner, repo, page, url.QueryEscape(">="+sinceStr))

		body, code, err := c.get(apiPath)
		if err != nil {
			return 0, err
		}
		if code != http.StatusOK {
			return 0, fmt.Errorf("GitHub Actions API returned %d", code)
		}

		var listResult struct {
			TotalCount   int `json:"total_count"`
			WorkflowRuns []struct {
				ID int `json:"id"`
			} `json:"workflow_runs"`
		}
		if err := json.Unmarshal(body, &listResult); err != nil {
			return 0, fmt.Errorf("parsing workflow runs list: %w", err)
		}

		if len(listResult.WorkflowRuns) == 0 {
			break
		}

		for _, run := range listResult.WorkflowRuns {
			targeted, err := c.runTargetsBranch(owner, repo, run.ID, inputKey)
			if err != nil {
				continue
			}
			if targeted {
				count++
			}
		}

		if len(listResult.WorkflowRuns) < 100 {
			break
		}
		page++
	}

	return count, nil
}

func (c *GitHubClient) runTargetsBranch(owner, repo string, runID int, inputKey string) (bool, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID)

	body, code, err := c.get(apiPath)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("GitHub Actions API returned %d for run %d", code, runID)
	}

	var runDetail struct {
		Event string `json:"event"`
		// workflow_dispatch inputs are a map of string→string
		Inputs map[string]string `json:"inputs"`
	}
	if err := json.Unmarshal(body, &runDetail); err != nil {
		return false, fmt.Errorf("parsing run detail: %w", err)
	}

	if runDetail.Inputs == nil {
		return false, nil
	}
	return runDetail.Inputs[inputKey] == "true", nil
}

// branchToInputKey maps a branch name to its workflow_dispatch input key.
func branchToInputKey(branch string) string {
	switch branch {
	case "oadp-1.3":
		return "oadp_1_3"
	case "oadp-1.4":
		return "oadp_1_4"
	case "oadp-1.5":
		return "oadp_1_5"
	case "oadp-1.6":
		return "oadp_1_6"
	case "oadp-dev":
		return "oadp_dev"
	default:
		return ""
	}
}

func (c *GitHubClient) searchIssuesCount(query string) (int, error) {
	apiPath := "/search/issues?per_page=1&q=" + url.QueryEscape(query)

	body, code, err := c.get(apiPath)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK {
		return 0, fmt.Errorf("GitHub Search API returned %d", code)
	}

	var result struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing search response: %w", err)
	}
	return result.TotalCount, nil
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
