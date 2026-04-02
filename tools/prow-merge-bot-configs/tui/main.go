package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Use ContinueOnError so we can control the exit code for unknown flags.
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	branch := fs.String("branch", "main", "Branch of openshift/release to audit")
	localPath := fs.String("local", "", "Path to local openshift/release checkout")
	format := fs.String("format", "", "Output format: text, markdown, interactive, or json (auto-detects if empty)")
	noMouse := fs.Bool("no-mouse", false, "Disable mouse support")
	skipQueue := fs.Bool("skip-queue", false, "Skip merge queue checks")
	watch := fs.String("watch", "", "Auto-refresh interval (e.g. 5m, 30s). Implies interactive mode.")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	// Parse --watch interval if provided.
	var watchInterval time.Duration
	if *watch != "" {
		var err error
		watchInterval, err = time.ParseDuration(*watch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid --watch interval %q: %v\n", *watch, err)
			os.Exit(1)
		}
		if watchInterval <= 0 {
			fmt.Fprintf(os.Stderr, "Error: --watch interval must be positive\n")
			os.Exit(1)
		}
	}

	// Validate --local path if provided.
	if *localPath != "" {
		checkDir := *localPath + "/core-services/prow/02_config"
		if fi, err := os.Stat(checkDir); err != nil || !fi.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: --local path does not look like an openshift/release checkout\n")
			fmt.Fprintf(os.Stderr, "Expected to find: %s\n", checkDir)
			os.Exit(1)
		}
	}

	// Check for gh CLI — needed for merge queue and token detection.
	ghAvailable := true
	if _, err := exec.LookPath("gh"); err != nil {
		ghAvailable = false
		fmt.Fprintf(os.Stderr, "Warning: gh CLI not found — merge queue checks will be unavailable\n")
		fmt.Fprintf(os.Stderr, "  Install: https://cli.github.com/\n")
	}

	// Token detection: check env, then try gh auth token.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" && ghAvailable {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err == nil {
			token = strings.TrimSpace(string(out))
		}
	}
	if token == "" {
		fmt.Fprintf(os.Stderr, "Warning: no GitHub token found — API rate limits will be restrictive\n")
		fmt.Fprintf(os.Stderr, "  Set GITHUB_TOKEN or run: gh auth login\n")
	}

	// Auto-detect format: interactive if terminal, text otherwise.
	// --watch implies interactive mode.
	outputFormat := *format
	if watchInterval > 0 {
		outputFormat = "interactive"
	} else if outputFormat == "" {
		fi, _ := os.Stdout.Stat()
		if fi != nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			outputFormat = "interactive"
		} else {
			outputFormat = "text"
		}
	}

	switch outputFormat {
	case "text":
		report, _, err := RunAudit(*branch, *localPath, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(RenderText(report))

	case "markdown":
		report, _, err := RunAudit(*branch, *localPath, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(RenderMarkdown(report))

	case "json":
		report, _, err := RunAudit(*branch, *localPath, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, err := RenderJSON(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(output)

	default:
		// Interactive TUI mode.
		m := newModel(*branch, *localPath, token, *skipQueue, ghAvailable, watchInterval)

		opts := []tea.ProgramOption{tea.WithAltScreen()}
		if !*noMouse {
			opts = append(opts, tea.WithMouseCellMotion())
		}

		p := tea.NewProgram(m, opts...)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	}
}
