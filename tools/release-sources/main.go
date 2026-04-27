package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var jsonOutput bool
	var mdOutput bool

	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.BoolVar(&mdOutput, "md", false, "Output as Markdown (for wiki pages)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: release-sources [flags] <branch>\n\n")
		fmt.Fprintf(os.Stderr, "Compare OADP image definitions across release sources.\n\n")
		fmt.Fprintf(os.Stderr, "Sources checked:\n")
		fmt.Fprintf(os.Stderr, "  1. Pyxis repo config (oadp.yaml)     — GitLab (gitlab.cee.redhat.com)\n")
		fmt.Fprintf(os.Stderr, "  2. ocp-build-data images             — GitHub\n")
		fmt.Fprintf(os.Stderr, "  3. oadp-operator image-references    — GitHub\n")
		fmt.Fprintf(os.Stderr, "  4. Konflux release-data stage        — GitLab (gitlab.cee.redhat.com)\n")
		fmt.Fprintf(os.Stderr, "  5. Konflux release-data prod         — GitLab (gitlab.cee.redhat.com)\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  release-sources oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  release-sources oadp-1.5\n")
		fmt.Fprintf(os.Stderr, "  release-sources --json oadp-1.6\n")
		fmt.Fprintf(os.Stderr, "  release-sources --md oadp-1.6 > wiki.md\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	branch := flag.Arg(0)

	// Create clients
	gh, err := NewGitHubClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	gl := NewGitLabClient(pyxisGitLab)

	// Header
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s%sFetching sources for %s...%s\n", cBold, cCyan, branch, cReset)
	fmt.Fprintln(os.Stderr)

	// Fetch all sources
	src := FetchAll(gh, gl, branch)

	// Build repo union
	repos := BuildUnion(src)

	// Find issues
	issues := FindIssues(src, repos)

	// Render
	if jsonOutput {
		RenderJSON(os.Stdout, branch, src, repos, issues)
	} else if mdOutput {
		RenderMarkdown(os.Stdout, branch, src, repos, issues)
	} else {
		RenderTable(os.Stdout, branch, src, repos, issues)
	}

}
