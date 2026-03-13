package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var (
		configDir  string
		waveFilter int
		repoFilter string
		jsonOutput bool
		verbose    bool
		format     string
	)

	flag.StringVar(&configDir, "config-dir", "", "Path to rebase-configs/ (auto-detected if empty)")
	flag.IntVar(&waveFilter, "wave", 0, "Only check repos in this wave (0 = all)")
	flag.StringVar(&repoFilter, "repo", "", "Only check this repo (short name, e.g. 'velero')")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed check output")
	flag.StringVar(&format, "format", "table", "Output format: table or text")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rebase-status [flags] <branch>\n\n")
		fmt.Fprintf(os.Stderr, "Check OADP rebase readiness for a given branch.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --wave 3\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --repo velero\n")
		fmt.Fprintf(os.Stderr, "  rebase-status --format text oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  rebase-status oadp-1.6 --json\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	branch := flag.Arg(0)

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

	// Run checks
	results := RunAllChecks(specs, DefaultChecks, client)

	// Render output
	if jsonOutput {
		RenderJSON(os.Stdout, results)
	} else if format == "text" {
		RenderText(os.Stdout, results, DefaultChecks, branch)
	} else {
		RenderTable(os.Stdout, results, DefaultChecks, branch)
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
