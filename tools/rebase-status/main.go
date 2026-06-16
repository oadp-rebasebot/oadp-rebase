package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
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
	)

	flag.StringVar(&configDir, "config-dir", "", "Path to rebase-configs/ (auto-detected if empty)")
	flag.IntVar(&waveFilter, "wave", 0, "Only check repos in this wave (0 = all)")
	flag.StringVar(&repoFilter, "repo", "", "Only check this repo (short name, e.g. 'velero')")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed check output")
	flag.StringVar(&format, "format", "table", "Output format: table, text, or markdown")
	flag.BoolVar(&hideDepDetails, "hide-dependency-details", false, "Hide commits between current and target hash for out-of-sync dependencies")
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

	// Find config directory
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
		var branchResults []BranchResult
		for _, branch := range flag.Args() {
			clearStores()

			specs, err := LoadSpecs(configDir, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: %v\n", branch, err)
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

			branchResults = append(branchResults, BranchResult{Branch: branch, Statuses: statuses})
		}
		RenderHome(os.Stdout, branchResults)
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

	// Render output
	if jsonOutput {
		RenderJSON(os.Stdout, results)
	} else if format == "text" {
		RenderText(os.Stdout, results, branch)
	} else if format == "markdown" || format == "md" {
		RenderMarkdown(os.Stdout, results, branch)
	} else {
		RenderTable(os.Stdout, results, branch)
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
