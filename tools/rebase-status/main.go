package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	var (
		configDir      string
		waveFilter     int
		repoFilter     string
		jsonOutput     bool
		verbose        bool
		format         string
		hideDepDetails bool
		tallyFile      string
	)

	flag.StringVar(&configDir, "config-dir", "", "Path to rebase-configs/ (auto-detected if empty)")
	flag.IntVar(&waveFilter, "wave", 0, "Only check repos in this wave (0 = all)")
	flag.StringVar(&repoFilter, "repo", "", "Only check this repo (short name, e.g. 'velero')")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed check output")
	flag.StringVar(&format, "format", "table", "Output format: table, text, or markdown")
	flag.BoolVar(&hideDepDetails, "hide-dependency-details", false, "Hide commits between current and target hash for out-of-sync dependencies")
	flag.StringVar(&tallyFile, "tally-file", "", "Path to pr-tallies.json for PR opened/merged counts (used with --format home)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rebase-status [flags] <branch>\n\n")
		fmt.Fprintf(os.Stderr, "Check OADP rebase readiness for a given branch.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --wave 3\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --repo velero\n")
		fmt.Fprintf(os.Stderr, "  rebase-status --format text oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  rebase-status --format markdown oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --json\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	reposYAML, yamlErr := FindReposYAML()
	if yamlErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", yamlErr)
		fmt.Fprintf(os.Stderr, "hint: run from the oadp-rebase repo root\n")
		os.Exit(1)
	}
	if yamlErr = LoadReposYAML(reposYAML); yamlErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", yamlErr)
		os.Exit(1)
	}

	if configDir == "" {
		var err error
		configDir, err = FindConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			fmt.Fprintf(os.Stderr, "hint: run from the oadp-rebase repo root, or use --config-dir\n")
			os.Exit(1)
		}
	}

	// Create GitHub client
	client, err := NewGitHubClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Check rate limit
	if verbose {
		remaining, err := client.RateLimitRemaining()
		if err == nil {
			fmt.Fprintf(os.Stderr, "GitHub API rate limit remaining: %d\n", remaining)
		}
	}

	// Create Quay client for image checks
	quayClient = NewQuayClient()

	// Home page mode: multiple branches → single Home.md
	if format == "home" {
		tallies := loadTallies(tallyFile)

		var branchResults []BranchResult
		var failedBranches []string
		for _, branch := range flag.Args() {
			clearStores()

			specs, err := LoadSpecs(configDir, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s: %v\n", branch, err)
				failedBranches = append(failedBranches, branch)
				continue
			}
			specs = filterSpecs(specs, waveFilter, repoFilter)

			rd, rdErr := FetchReleaseData(client, branch)
			if rdErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: release data: %v\n", branch, rdErr)
			}
			currentReleaseData = rd

			statuses := RunAllChecks(specs, DefaultChecks, client)
			if !hideDepDetails {
				fetchDepCommitDetails(statuses, client, branch)
			}

			var veleroTagAlign *VeleroTagAlignment
			versionsVars, vErr := LoadVersionsVars(configDir, branch)
			if vErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: versions: %v\n", branch, vErr)
			}
			veleroTagAlign = CheckVeleroTagAlignment(statuses, versionsVars, client)

			cvePRs := fetchCVEPRs(client, specs)

			// Compute PR tallies for this branch
			var tallyResult *PRTallyResult
			if tallyFile != "" {
				tallyResult = computeTally(client, branch, tallies, statuses, cvePRs)
			}

			branchResults = append(branchResults, BranchResult{Branch: branch, Statuses: statuses, VeleroTagAlign: veleroTagAlign, CVEPRs: cvePRs, Tally: tallyResult})
		}
		if tallyFile != "" {
			RenderHome(os.Stdout, branchResults, tallies)
			saveTallies(tallyFile, tallies)
		} else {
			RenderHome(os.Stdout, branchResults)
		}
		if len(failedBranches) > 0 {
			fmt.Fprintf(os.Stderr, "warning: failed to load %d branch(es): %v\n", len(failedBranches), failedBranches)
		}
		return
	}

	// Single-branch mode
	branch := flag.Arg(0)

	// Load repo specs
	specs, err := LoadSpecs(configDir, branch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading configs: %v\n", err)
		os.Exit(1)
	}

	// Apply filters
	specs = filterSpecs(specs, waveFilter, repoFilter)
	if len(specs) == 0 {
		fmt.Fprintf(os.Stderr, "no repos matched filters (wave=%d, repo=%q)\n", waveFilter, repoFilter)
		os.Exit(1)
	}

	// Fetch release data (image-references + ocp-build-data)
	rd, rdErr := FetchReleaseData(client, branch)
	if rdErr != nil {
		fmt.Fprintf(os.Stderr, "warning: release data: %v\n", rdErr)
	}
	currentReleaseData = rd

	// Run checks
	results := RunAllChecks(specs, DefaultChecks, client)

	// Fetch dependency commit details (default: on, unless --hide-dependency-details)
	if !hideDepDetails {
		fetchDepCommitDetails(results, client, branch)
	}

	// Velero tag alignment check
	var veleroTagAlign *VeleroTagAlignment
	versionsVars, vErr := LoadVersionsVars(configDir, branch)
	if vErr != nil {
		fmt.Fprintf(os.Stderr, "warning: versions: %v\n", vErr)
	}
	veleroTagAlign = CheckVeleroTagAlignment(results, versionsVars, client)

	// Fetch CVE PRs
	cvePRs := fetchCVEPRs(client, specs)

	// Render output
	if jsonOutput {
		RenderJSON(os.Stdout, results, veleroTagAlign, cvePRs)
	} else if format == "text" {
		RenderText(os.Stdout, results, branch, veleroTagAlign)
	} else if format == "markdown" || format == "md" {
		RenderMarkdown(os.Stdout, results, branch, veleroTagAlign, cvePRs)
	} else {
		RenderTable(os.Stdout, results, branch, veleroTagAlign)
	}

	// Exit with error code if any failures
	for _, r := range results {
		for _, iss := range r.Issues {
			if iss.Severity == "error" {
				os.Exit(1)
			}
		}
	}
}

// fetchDepCommitDetails populates DepSync.Commits for out-of-sync dependencies
// by querying the GitHub compare API. The branch parameter is the downstream
// branch (e.g. "oadp-1.6") which is the branch these commits live on.
func fetchDepCommitDetails(results []RepoStatus, client *GitHubClient, branch string) {
	for i := range results {
		for j := range results[i].DepSyncs {
			ds := &results[i].DepSyncs[j]
			if ds.InSync || ds.HaveHash == "" || ds.HeadHash == "" {
				continue
			}
			commits, err := client.CompareCommits(ds.Org, ds.Repo, ds.HaveHash, ds.HeadHash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not fetch commits for %s/%s (%s..%s): %v\n",
					ds.Org, ds.Repo, short(ds.HaveHash), short(ds.HeadHash), err)
				continue
			}
			ds.Commits = commits
		}
	}
}

func fetchCVEPRs(client *GitHubClient, specs []RepoSpec) []CVEPRInfo {
	type result struct {
		prs []CVEPRInfo
	}
	results := make([]result, len(specs))
	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func(i int, spec RepoSpec) {
			defer wg.Done()
			prs, err := client.SearchCVEPRs(spec.Org, spec.Repo, spec.Branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: CVE PR search for %s/%s: %v\n", spec.Org, spec.Repo, err)
				return
			}
			results[i] = result{prs: prs}
		}(i, spec)
	}
	wg.Wait()

	var all []CVEPRInfo
	for _, r := range results {
		all = append(all, r.prs...)
	}
	return all
}

func filterSpecs(specs []RepoSpec, wave int, repo string) []RepoSpec {
	var filtered []RepoSpec
	for _, s := range specs {
		if wave > 0 && s.Wave != wave {
			continue
		}
		if repo != "" && !strings.Contains(s.Repo, repo) {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func loadTallies(path string) map[string]*PRTally {
	tallies := make(map[string]*PRTally)
	if path == "" {
		return tallies
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist yet — start fresh
		return tallies
	}
	if err := json.Unmarshal(data, &tallies); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not parse %s: %v (starting fresh)\n", path, err)
		return make(map[string]*PRTally)
	}
	return tallies
}

func saveTallies(path string, tallies map[string]*PRTally) {
	data, err := json.MarshalIndent(tallies, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not marshal tallies: %v\n", err)
		return
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: could not write %s: %v\n", path, err)
	}
}

const rebaseRepoOwner = "oadp-rebasebot"
const rebaseRepoName = "oadp-rebase"

// computeTally queries GitHub for PR counts and handles reset logic.
// It returns the tally result for display and updates the tallies map in place.
func computeTally(client *GitHubClient, branch string, tallies map[string]*PRTally, statuses []RepoStatus, cvePRs []CVEPRInfo) *PRTallyResult {
	tally, ok := tallies[branch]
	if !ok {
		tally = &PRTally{ResetAt: time.Now().UTC()}
		tallies[branch] = tally
	}

	opened, merged, err := client.SearchRebasePRCounts(branch, tally.ResetAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: PR tally search: %v\n", branch, err)
		return nil
	}

	triggered, err := client.CountBranchWorkflowRuns(rebaseRepoOwner, rebaseRepoName, branch, tally.ResetAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: workflow trigger count: %v\n", branch, err)
		triggered = 0
	}

	result := &PRTallyResult{
		Triggered: triggered,
		Opened:    opened,
		Merged:    merged,
		ResetAt:   tally.ResetAt,
	}

	// Check reset condition: all repos ready, no errors, no CVE PRs
	total, ready, errs, _ := scoreboard(statuses)
	if total > 0 && ready == total && errs == 0 && len(cvePRs) == 0 {
		// Record the completed cycle in history before resetting
		if opened > 0 || merged > 0 || triggered > 0 {
			score := fmt.Sprintf("%d/%d repos ready (100%%)", total, total)
			tally.History = append(tally.History, PRCycleHistory{
				Date:      time.Now().UTC().Format("2006-01-02"),
				Score:     score,
				Triggered: triggered,
				Opened:    opened,
				Merged:    merged,
			})
		}
		tally.ResetAt = time.Now().UTC()
	}

	return result
}
