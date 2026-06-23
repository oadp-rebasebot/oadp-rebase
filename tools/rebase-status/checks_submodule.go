package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SubmoduleInfo holds parsed .gitmodules data for one submodule.
type SubmoduleInfo struct {
	Path   string
	URL    string
	Branch string
}

// submoduleRepos maps submodule URLs (with and without .git suffix) to their
// GitHub org/repo. Add entries here when onboarding repos with submodule deps.
var submoduleRepos = map[string]struct{ org, repo string }{
	"https://github.com/openshift/velero":     {"openshift", "velero"},
	"https://github.com/openshift/velero.git": {"openshift", "velero"},
	"https://github.com/openshift/restic":     {"openshift", "restic"},
	"https://github.com/openshift/restic.git": {"openshift", "restic"},
	"https://github.com/migtools/kopia":       {"migtools", "kopia"},
	"https://github.com/migtools/kopia.git":   {"migtools", "kopia"},
}

// parseGitmodules parses a .gitmodules file and returns a map of path → SubmoduleInfo.
func parseGitmodules(content string) map[string]SubmoduleInfo {
	subs := map[string]SubmoduleInfo{}
	var current SubmoduleInfo
	var currentName string

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "[submodule ") {
			if currentName != "" {
				subs[current.Path] = current
			}
			currentName = strings.Trim(strings.TrimPrefix(line, "[submodule "), "\"]")
			current = SubmoduleInfo{}
			continue
		}

		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			switch key {
			case "path":
				current.Path = val
			case "url":
				current.URL = val
			case "branch":
				current.Branch = val
			}
		}
	}
	if currentName != "" && current.Path != "" {
		subs[current.Path] = current
	}

	return subs
}

// parseSubmoduleDeps combines .gitmodules content with git tree entries (submodule SHAs)
// to produce DepSync entries for known downstream submodules. treeEntries maps
// submodule path → pinned commit SHA (full 40-char). selfRepo is "org/repo" to
// skip self-references.
func parseSubmoduleDeps(gitmodules string, treeEntries map[string]string, selfRepo string) []DepSync {
	subs := parseGitmodules(gitmodules)
	var syncs []DepSync

	for path, sub := range subs {
		info, ok := submoduleRepos[sub.URL]
		if !ok {
			continue
		}

		key := info.org + "/" + info.repo
		if key == selfRepo {
			continue
		}

		sha, ok := treeEntries[path]
		if !ok {
			continue
		}

		// Use first 12 chars to match go.mod pseudo-version convention
		haveHash := sha
		if len(haveHash) > 12 {
			haveHash = haveHash[:12]
		}

		syncs = append(syncs, DepSync{
			Module:          sub.URL,
			Org:             info.org,
			Repo:            info.repo,
			HaveHash:        haveHash,
			SubmoduleBranch: sub.Branch,
		})
	}

	return syncs
}

// SubmoduleEntries fetches the git tree for a branch and returns entries with
// type "commit" (git submodules). Returns a map of path → full SHA.
func (c *GitHubClient) SubmoduleEntries(org, repo, branch string) (map[string]string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/git/trees/%s", org, repo, branch)

	body, code, err := c.get(apiPath)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("GitHub API %s returned %d", apiPath, code)
	}

	var result struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing tree: %w", err)
	}

	entries := map[string]string{}
	for _, e := range result.Tree {
		if e.Type == "commit" {
			entries[e.Path] = e.SHA
		}
	}
	return entries, nil
}
