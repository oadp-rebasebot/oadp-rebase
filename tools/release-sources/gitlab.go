package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// GitLabClient fetches files from GitLab via the glab CLI.
type GitLabClient struct {
	hostname string
}

// NewGitLabClient creates a client for the given GitLab hostname.
func NewGitLabClient(hostname string) *GitLabClient {
	return &GitLabClient{hostname: hostname}
}

// FileContent fetches a file's raw content from a GitLab project.
// project is the full path (e.g. "releng/pyxis-repo-configs").
// path is the file path within the repo.
// ref is the branch/tag.
func (c *GitLabClient) FileContent(project, path, ref string) ([]byte, error) {
	projectEnc := url.PathEscape(project)
	pathEnc := url.PathEscape(path)

	endpoint := fmt.Sprintf("projects/%s/repository/files/%s/raw?ref=%s", projectEnc, pathEnc, ref)

	out, err := exec.Command("glab", "api", "--hostname", c.hostname, endpoint).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("glab api %s: %s", endpoint, strings.TrimSpace(string(out)))
	}
	return out, nil
}

