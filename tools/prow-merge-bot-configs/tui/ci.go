package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// CIOperatorTest is a single test from a ci-operator config.
type CIOperatorTest struct {
	Name string // "as" field from the config
}

// CIOperatorBranch groups tests by branch extracted from ci-operator configs.
type CIOperatorBranch struct {
	Branch string
	Tests  []CIOperatorTest
}

// CIInfo holds CI configuration details for a single repository.
type CIInfo struct {
	Repo                 string
	GoVersion            string // inferred Go version
	HasProw              bool
	HasCIOperator        bool // true if ci-operator configs exist in openshift/release
	CIOperatorConfigCount int  // number of ci-operator config files found
	CIOperatorBranches   []CIOperatorBranch // parsed ci-operator configs grouped by branch
	HasActions           bool
	ProwJobs             []ProwJob
	ActionsWflows        []ActionsWorkflow
	Error                string // non-empty if fetch failed or timed out
}

// ProwJob represents a single Prow CI job from presubmits/postsubmits.
type ProwJob struct {
	Name       string // e.g. pull-ci-velero-e2e-aws
	Type       string // "presubmit" or "postsubmit"
	GoVersion  string // extracted from builder image tag, if any
	ImageRef   string // container image reference
	AlwaysRun  bool
	RunIfChanged string
}

// ActionsWorkflow represents a GitHub Actions workflow file.
type ActionsWorkflow struct {
	Filename  string   // e.g. unit-tests.yaml
	Name      string   // workflow name from YAML
	Triggers  []string // e.g. ["push", "pull_request"]
	GoVersion string   // from actions/setup-go
}

// ciCache caches CI info per repo.
var ciCache struct {
	sync.Mutex
	data map[string]*CIInfo
}

// ClearCICache clears cached CI info for a specific repo, or all if repo is empty.
func ClearCICache(repo string) {
	ciCache.Lock()
	defer ciCache.Unlock()
	if repo == "" {
		ciCache.data = nil
	} else if ciCache.data != nil {
		delete(ciCache.data, repo)
	}
}

// FetchAllCIInfo fetches CI configuration for all repos concurrently.
// Uses cached Prow configs from the audit and fetches Actions workflows separately.
// Each repo fetch is bounded by the given timeout duration.
// localPath and branch are used to check for ci-operator configs in openshift/release.
func FetchAllCIInfo(repos []string, configs map[string]*RepoConfig, localPath, branch, token string, timeout time.Duration) map[string]*CIInfo {
	ciCache.Lock()
	if ciCache.data == nil {
		ciCache.data = make(map[string]*CIInfo)
	}
	ciCache.Unlock()

	results := make(map[string]*CIInfo, len(repos))
	var mu sync.Mutex

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, repo := range repos {
		ciCache.Lock()
		if cached, ok := ciCache.data[repo]; ok {
			ciCache.Unlock()
			mu.Lock()
			results[repo] = cached
			mu.Unlock()
			continue
		}
		ciCache.Unlock()

		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			rc := configs[repo]
			ci := buildCIInfo(ctx, repo, rc, localPath, token)
			if ctx.Err() != nil {
				ci.Error = "timeout"
			}

			ciCache.Lock()
			ciCache.data[repo] = ci
			ciCache.Unlock()

			mu.Lock()
			results[repo] = ci
			mu.Unlock()
		}(repo)
	}

	wg.Wait()
	return results
}

// buildCIInfo constructs CIInfo for a single repo.
func buildCIInfo(ctx context.Context, repo string, rc *RepoConfig, localPath, token string) *CIInfo {
	ci := &CIInfo{Repo: repo}

	// Extract Prow jobs from the already-fetched _prowconfig.yaml.
	if rc != nil && rc.ProwConfig != nil {
		ci.ProwJobs = extractProwJobs(rc.ProwConfig, repo)
		ci.HasProw = len(ci.ProwJobs) > 0
	}

	// Check for ci-operator configs if no explicit Prow jobs were found.
	if !ci.HasProw {
		ci.CIOperatorBranches = fetchCIOperatorConfigs(ctx, repo, localPath, token)
		ci.HasCIOperator = len(ci.CIOperatorBranches) > 0
		ci.CIOperatorConfigCount = len(ci.CIOperatorBranches)
		ci.HasProw = ci.HasCIOperator
	}

	// Fetch GitHub Actions workflows.
	ci.ActionsWflows = fetchActionsWorkflows(ctx, repo, token)
	ci.HasActions = len(ci.ActionsWflows) > 0

	// Determine Go version (priority: Prow builder image > Actions setup-go > go.mod).
	ci.GoVersion = inferGoVersion(ctx, ci, repo, token)

	return ci
}

// extractProwJobs parses presubmits and postsubmits from the Prow config.
func extractProwJobs(prowConfig map[string]interface{}, repo string) []ProwJob {
	var jobs []ProwJob

	for _, section := range []string{"presubmits", "postsubmits"} {
		jobType := "presubmit"
		if section == "postsubmits" {
			jobType = "postsubmit"
		}

		sectionVal, ok := prowConfig[section]
		if !ok {
			continue
		}
		sectionMap, ok := sectionVal.(map[string]interface{})
		if !ok {
			continue
		}

		repoJobs, ok := sectionMap[repo]
		if !ok {
			continue
		}
		jobList, ok := repoJobs.([]interface{})
		if !ok {
			continue
		}

		for _, j := range jobList {
			jMap, ok := j.(map[string]interface{})
			if !ok {
				continue
			}

			pj := ProwJob{Type: jobType}
			if name, ok := jMap["name"]; ok {
				pj.Name = toString(name)
			}
			if ar, ok := jMap["always_run"]; ok {
				pj.AlwaysRun = toString(ar) == "true"
			}
			if ric, ok := jMap["run_if_changed"]; ok {
				pj.RunIfChanged = toString(ric)
			}

			// Extract container image for Go version detection.
			pj.ImageRef, pj.GoVersion = extractBuilderImage(jMap)

			jobs = append(jobs, pj)
		}
	}

	return jobs
}

// fetchCIOperatorConfigs fetches and parses ci-operator config files for the repo
// from openshift/release, returning tests grouped by branch.
func fetchCIOperatorConfigs(ctx context.Context, repo, localPath, token string) []CIOperatorBranch {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	org, repoName := parts[0], parts[1]

	if localPath != "" {
		return fetchCIOperatorConfigsLocal(org, repoName, localPath)
	}
	return fetchCIOperatorConfigsGitHub(ctx, org, repoName, token)
}

// extractBranchFromFilename extracts the branch name from a ci-operator config filename.
// Files are named {org}-{repo}-{branch}.yaml, e.g. "openshift-velero-release-4.16.yaml".
func extractBranchFromFilename(org, repo, filename string) string {
	// Strip extension.
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	prefix := org + "-" + repo + "-"
	if strings.HasPrefix(name, prefix) {
		return name[len(prefix):]
	}
	// Fallback: use filename without extension if prefix doesn't match.
	return name
}

// parseCIOperatorTests parses a ci-operator config YAML and extracts test names.
func parseCIOperatorTests(data []byte) []CIOperatorTest {
	var parsed struct {
		Tests []struct {
			As string `yaml:"as"`
		} `yaml:"tests"`
		Images []struct {
			To string `yaml:"to"`
		} `yaml:"images"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}

	var tests []CIOperatorTest
	for _, t := range parsed.Tests {
		if t.As != "" {
			tests = append(tests, CIOperatorTest{Name: t.As})
		}
	}
	for _, img := range parsed.Images {
		if img.To != "" {
			tests = append(tests, CIOperatorTest{Name: "[image] " + img.To})
		}
	}
	return tests
}

// fetchCIOperatorConfigsLocal reads ci-operator configs from a local openshift/release checkout.
func fetchCIOperatorConfigsLocal(org, repo, localPath string) []CIOperatorBranch {
	dir := filepath.Join(localPath, "ci-operator", "config", org, repo)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var branches []CIOperatorBranch
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		branch := extractBranchFromFilename(org, repo, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			branches = append(branches, CIOperatorBranch{Branch: branch})
			continue
		}
		tests := parseCIOperatorTests(data)
		branches = append(branches, CIOperatorBranch{Branch: branch, Tests: tests})
	}

	sortCIOperatorBranches(branches)
	return branches
}

// fetchCIOperatorConfigsGitHub fetches ci-operator configs via the GitHub API.
func fetchCIOperatorConfigsGitHub(ctx context.Context, org, repo, token string) []CIOperatorBranch {
	dirURL := fmt.Sprintf("https://api.github.com/repos/openshift/release/contents/ci-operator/config/%s/%s", org, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", dirURL, nil)
	if err != nil {
		return nil
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := yaml.Unmarshal(body, &entries); err != nil {
		return nil
	}

	var branches []CIOperatorBranch
	for _, e := range entries {
		if e.Type != "file" || (!strings.HasSuffix(e.Name, ".yaml") && !strings.HasSuffix(e.Name, ".yml")) {
			continue
		}
		branch := extractBranchFromFilename(org, repo, e.Name)
		tests := fetchCIOperatorFileGitHub(ctx, org, repo, e.Name, token)
		branches = append(branches, CIOperatorBranch{Branch: branch, Tests: tests})
	}

	sortCIOperatorBranches(branches)
	return branches
}

// fetchCIOperatorFileGitHub fetches and parses a single ci-operator config file from GitHub.
func fetchCIOperatorFileGitHub(ctx context.Context, org, repo, filename, token string) []CIOperatorTest {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/openshift/release/main/ci-operator/config/%s/%s/%s", org, repo, filename)
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return parseCIOperatorTests(data)
}

// sortCIOperatorBranches sorts branches alphabetically.
func sortCIOperatorBranches(branches []CIOperatorBranch) {
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Branch < branches[j].Branch
	})
}

// builderImageRe matches konveyor/builder image tags like ubi9-v1.22.
var builderImageRe = regexp.MustCompile(`quay\.io/konveyor/builder:[\w-]*v(\d+\.\d+)`)

// extractBuilderImage finds a konveyor/builder image reference in a job spec
// and extracts the Go version from the tag.
func extractBuilderImage(jobMap map[string]interface{}) (imageRef, goVersion string) {
	// Navigate spec.containers[].image
	specVal, ok := jobMap["spec"]
	if !ok {
		return "", ""
	}
	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "", ""
	}
	containersVal, ok := specMap["containers"]
	if !ok {
		return "", ""
	}
	containers, ok := containersVal.([]interface{})
	if !ok {
		return "", ""
	}

	for _, c := range containers {
		cMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		img, ok := cMap["image"]
		if !ok {
			continue
		}
		imgStr := toString(img)
		imageRef = imgStr
		if matches := builderImageRe.FindStringSubmatch(imgStr); len(matches) >= 2 {
			goVersion = matches[1]
			return imageRef, goVersion
		}
	}
	return imageRef, ""
}

// fetchActionsWorkflows fetches GitHub Actions workflow files from a repo.
func fetchActionsWorkflows(ctx context.Context, repo string, token string) []ActionsWorkflow {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil
	}

	// Try to list workflows directory via GitHub API (trees endpoint).
	// Fall back to known common workflow filenames.
	workflowFiles := discoverWorkflowFiles(ctx, parts[0], parts[1], token)
	if len(workflowFiles) == 0 {
		return nil
	}

	var workflows []ActionsWorkflow
	for _, filename := range workflowFiles {
		wf := fetchSingleWorkflow(ctx, parts[0], parts[1], filename, token)
		if wf != nil {
			workflows = append(workflows, *wf)
		}
	}
	return workflows
}

// discoverWorkflowFiles lists .github/workflows/*.yaml and *.yml files in a repo.
func discoverWorkflowFiles(ctx context.Context, org, repo, token string) []string {
	// Use GitHub API to list the tree at .github/workflows/
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/.github/workflows", org, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	// Parse the JSON array of file entries.
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := yaml.Unmarshal(body, &entries); err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if strings.HasSuffix(e.Name, ".yaml") || strings.HasSuffix(e.Name, ".yml") {
			files = append(files, e.Name)
		}
	}
	return files
}

// fetchSingleWorkflow fetches and parses a single workflow YAML file.
func fetchSingleWorkflow(ctx context.Context, org, repo, filename, token string) *ActionsWorkflow {
	url := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/HEAD/.github/workflows/%s",
		org, repo, filename,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}

	wf := &ActionsWorkflow{Filename: filename}

	// Extract workflow name.
	if name, ok := parsed["name"]; ok {
		wf.Name = toString(name)
	} else {
		wf.Name = filename
	}

	// Extract triggers from the "on" key.
	wf.Triggers = extractTriggers(parsed)

	// Extract Go version from actions/setup-go.
	wf.GoVersion = extractActionsGoVersion(data)

	return wf
}

// extractTriggers extracts trigger event names from workflow "on" field.
func extractTriggers(parsed map[string]interface{}) []string {
	// The "on" key can be: string, list, or map.
	// yaml.v3 parses bare "on" as boolean true in some contexts,
	// but with map[string]interface{} it stays as string key "on".
	onVal, ok := parsed["on"]
	if !ok {
		return nil
	}

	switch v := onVal.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var triggers []string
		for _, t := range v {
			triggers = append(triggers, toString(t))
		}
		return triggers
	case map[string]interface{}:
		var triggers []string
		for key := range v {
			triggers = append(triggers, toString(key))
		}
		return triggers
	}
	return nil
}

// setupGoVersionRe matches version strings in actions/setup-go usage.
var setupGoVersionRe = regexp.MustCompile(`go-version[:\s]*['"]?(\d+\.\d+[\.\d]*)['"]?`)

// extractActionsGoVersion looks for actions/setup-go and extracts the Go version.
func extractActionsGoVersion(data []byte) string {
	content := string(data)

	// Check if actions/setup-go is used.
	if !strings.Contains(content, "actions/setup-go") {
		return ""
	}

	// Look for go-version setting.
	if matches := setupGoVersionRe.FindStringSubmatch(content); len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// inferGoVersion determines the Go version for a repo using multiple sources.
func inferGoVersion(ctx context.Context, ci *CIInfo, repo, token string) string {
	// Priority 1: Prow builder image tag.
	for _, job := range ci.ProwJobs {
		if job.GoVersion != "" {
			return job.GoVersion
		}
	}

	// Priority 2: GitHub Actions setup-go version.
	for _, wf := range ci.ActionsWflows {
		if wf.GoVersion != "" {
			return wf.GoVersion
		}
	}

	// Priority 3: go.mod from the repo.
	return fetchGoModVersion(ctx, repo, token)
}

// goModVersionRe matches "go X.Y" directive in go.mod.
var goModVersionRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)

// fetchGoModVersion fetches go.mod from a repo and extracts the Go version.
func fetchGoModVersion(ctx context.Context, repo, token string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return ""
	}

	url := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/HEAD/go.mod",
		parts[0], parts[1],
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	if matches := goModVersionRe.FindSubmatch(data); len(matches) >= 2 {
		return string(matches[1])
	}
	return ""
}
