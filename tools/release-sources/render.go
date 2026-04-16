package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ANSI color codes
var colorEnabled = isTerminal()

func ansi(code string) string {
	if colorEnabled {
		return code
	}
	return ""
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

var (
	cReset  = ansi("\033[0m")
	cBold   = ansi("\033[1m")
	cDim    = ansi("\033[2m")
	cRed    = ansi("\033[31m")
	cGreen  = ansi("\033[32m")
	cYellow = ansi("\033[33m")
	cCyan   = ansi("\033[36m")
)

// RenderTable prints the comparison matrix to the writer.
func RenderTable(w io.Writer, branch string, src *Sources, repos []string, issues []Issue) {
	// Header
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%sOADP Release Source Comparison%s\n", cBold, cCyan, cReset)
	fmt.Fprintf(w, "%s%s%s\n", cDim, time.Now().Format("2006-01-02 15:04 MST"), cReset)
	fmt.Fprintln(w, strings.Repeat("═", 70))

	// Sources summary
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%sSources — branch: %s%s\n", cBold, cCyan, branch, cReset)
	fmt.Fprintln(w, strings.Repeat("═", 70))

	skipped := len(exceptions)
	for _, sd := range []struct {
		data  *SourceData
		label string
	}{
		{&src.Pyxis, "Pyxis (oadp.yaml)"},
		{&src.OBD, "ocp-build-data"},
		{&src.ImageRefs, "image-references"},
		{&src.Stage, "Konflux stage"},
		{&src.Prod, "Konflux prod"},
	} {
		count := len(sd.data.Repos)
		avail := ""
		if !sd.data.Available {
			avail = fmt.Sprintf("  %s(unavailable)%s", cYellow, cReset)
		}
		fmt.Fprintf(w, "  %-21s %s%2d repos%s   %s%s%s%s\n",
			sd.label, cBold, count, cReset, cDim, sd.data.Detail, cReset, avail)
	}

	fmt.Fprintf(w, "  %s%-21s %2d repos%s", cDim, "Union", len(repos), cReset)
	if skipped > 0 {
		fmt.Fprintf(w, "   %s(%d skipped)%s", cDim, skipped, cReset)
	}
	fmt.Fprintln(w)

	// Pipeline diagram
	renderPipeline(w)

	// Comparison matrix
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%sComparison Matrix%s\n", cBold, cCyan, cReset)

	// Compute repo name column width
	nameWidth := 10
	fromWidth := 4
	for _, repo := range repos {
		if len(repo) > nameWidth {
			nameWidth = len(repo)
		}
		if from, ok := src.ImgRefFrom[repo]; ok {
			if len(from) > fromWidth {
				fromWidth = len(from)
			}
		}
	}

	V := fmt.Sprintf("%s│%s", cDim, cReset)

	// Header
	fmt.Fprintf(w, "  %3s  %-*s %s %-5s  %-5s  %-4s %s %-3s  %-8s %s %-6s  %-*s\n",
		"#", nameWidth, "Repository", V, "Pyxis", "Stage", "Prod", V, "OBD", "Delivery", V, "ImgRef", fromWidth, "From")

	// Separator
	sep := fmt.Sprintf("%s%s──%s─┼───%s──%s──%s──┼────%s───%s─────────┼───%s───────%s──%s",
		cDim,
		strings.Repeat("─", 3), strings.Repeat("─", nameWidth),
		strings.Repeat("─", 5), strings.Repeat("─", 5), strings.Repeat("─", 4),
		strings.Repeat("─", 3), strings.Repeat("─", 8),
		strings.Repeat("─", 6), strings.Repeat("─", fromWidth),
		cReset)
	fmt.Fprintf(w, "  %s\n", sep)

	// Rows
	prevPrefix := ""
	for i, repo := range repos {
		// Group separator
		curPrefix := repoGroupPrefix(repo)
		if prevPrefix != "" && curPrefix != prevPrefix {
			fmt.Fprintf(w, "  %s\n", sep)
		}
		prevPrefix = curPrefix

		// Source presence icons
		p := sourceIcon(src.Pyxis, repo, noPyxis[repo])
		s := sourceIcon(src.Stage, repo, false)
		pr := sourceIcon(src.Prod, repo, false)
		o := sourceIcon(src.OBD, repo, false)
		ir := sourceIcon(src.ImageRefs, repo, false)

		// Delivery check
		d := fmt.Sprintf("%s—%s", cDim, cReset)
		if delivery, ok := src.OBDDelivery[repo]; ok {
			if deliveryMatch(delivery, repo) {
				d = fmt.Sprintf("%s✅%s", cGreen, cReset)
			} else {
				d = fmt.Sprintf("%s❌%s", cRed, cReset)
			}
		}

		// From image (colored: registry dim, image cyan, tag yellow)
		fromDisplay := ""
		if from, ok := src.ImgRefFrom[repo]; ok {
			fromDisplay = colorFromImage(from)
		}

		// All-OK check for repo name highlighting
		allOK := isAllOK(src, repo)
		repoFmt := repo
		if allOK {
			repoFmt = fmt.Sprintf("%s%s%s", cGreen, repo, cReset)
		}

		fmt.Fprintf(w, "  %s%3d%s  %s%-*s %s %s     %s     %s    %s %s   %s        %s %s      %s\n",
			cDim, i+1, cReset,
			repoFmt, nameWidth-len(repo), "",
			V, p, s, pr,
			V, o, d,
			V, ir, fromDisplay)
	}
	fmt.Fprintf(w, "  %s\n", sep)

	// Issues
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%sIssues%s\n", cBold, cCyan, cReset)
	fmt.Fprintln(w, strings.Repeat("═", 70))

	errors := 0
	warnings := 0
	for _, iss := range issues {
		if iss.Severity == "error" {
			errors++
		} else {
			warnings++
		}
	}

	fmt.Fprintln(w)
	if len(issues) == 0 {
		fmt.Fprintf(w, "  %s%sNo issues found. All sources are in sync!%s ✅\n", cBold, cGreen, cReset)
	} else {
		fmt.Fprintf(w, "  %s%s%d errors%s%s, %s%d warnings%s\n",
			cBold, cRed, errors, cReset,
			cBold, cYellow, warnings, cReset)
		fmt.Fprintln(w)
		for _, iss := range issues {
			if iss.Severity == "error" {
				fmt.Fprintf(w, "  ❌  %-*s  %s%s%s", nameWidth, iss.Repo, cRed, iss.Message, cReset)
				if iss.InSources != "" {
					fmt.Fprintf(w, "  %s[in: %s]%s", cDim, iss.InSources, cReset)
				}
				fmt.Fprintln(w)
			}
		}
		for _, iss := range issues {
			if iss.Severity == "warning" {
				fmt.Fprintf(w, "  ⚠️   %-*s  %s%s%s\n", nameWidth, iss.Repo, cYellow, iss.Message, cReset)
			}
		}
	}

	// Score
	inSync := len(repos) - errors
	scoreColor := cRed
	if errors == 0 {
		scoreColor = cGreen
	} else if inSync*2 >= len(repos) {
		scoreColor = cYellow
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%sScore: %s%d/%d repos in sync", cBold, scoreColor, inSync, len(repos))
	if len(repos) > 0 {
		fmt.Fprintf(w, " (%d%%)", inSync*100/len(repos))
	}
	fmt.Fprintf(w, "%s\n\n", cReset)
}

// renderPipeline draws a diagram showing how the five sources relate.
func renderPipeline(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%sSource Pipeline%s\n", cBold, cCyan, cReset)
	fmt.Fprintln(w, strings.Repeat("═", 70))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %socp-build-data%s %s───┬──→%s %sPyxis%s          %s(container catalog)%s\n",
		cBold, cReset, cDim, cReset, cBold, cReset, cDim, cReset)
	fmt.Fprintf(w, "  %s(build configs)%s %s  ├──→%s %simage-refs%s     %s(operator bundle)%s\n",
		cDim, cReset, cDim, cReset, cBold, cReset, cDim, cReset)
	fmt.Fprintf(w, "                 %s└──→%s %sKonflux%s %s───┬──→%s %sStage%s  %s(advisory)%s\n",
		cDim, cReset, cBold, cReset, cDim, cReset, cBold, cReset, cDim, cReset)
	fmt.Fprintf(w, "                                  %s└──→%s %sProd%s   %s(advisory)%s\n",
		cDim, cReset, cBold, cReset, cDim, cReset)
}

// RenderJSON outputs the comparison as JSON.
func RenderJSON(w io.Writer, branch string, src *Sources, repos []string, issues []Issue) {
	type jsonSource struct {
		Name      string `json:"name"`
		Available bool   `json:"available"`
		Count     int    `json:"count"`
		Detail    string `json:"detail"`
	}

	type jsonRepo struct {
		Name      string `json:"name"`
		Pyxis     string `json:"pyxis"`
		OBD       string `json:"obd"`
		ImageRefs string `json:"image_refs"`
		Stage     string `json:"stage"`
		Prod      string `json:"prod"`
		Delivery  string `json:"delivery,omitempty"`
		From      string `json:"from,omitempty"`
	}

	type jsonIssue struct {
		Severity  string `json:"severity"`
		Repo      string `json:"repo"`
		Message   string `json:"message"`
		InSources string `json:"in_sources,omitempty"`
	}

	type jsonOutput struct {
		Branch  string       `json:"branch"`
		Time    string       `json:"time"`
		Sources []jsonSource `json:"sources"`
		Repos   []jsonRepo   `json:"repos"`
		Issues  []jsonIssue  `json:"issues"`
		Score   struct {
			InSync  int `json:"in_sync"`
			Total   int `json:"total"`
			Percent int `json:"percent"`
		} `json:"score"`
	}

	out := jsonOutput{
		Branch: branch,
		Time:   time.Now().Format(time.RFC3339),
	}

	for _, sd := range []*SourceData{&src.Pyxis, &src.OBD, &src.ImageRefs, &src.Stage, &src.Prod} {
		out.Sources = append(out.Sources, jsonSource{
			Name:      sd.Name,
			Available: sd.Available,
			Count:     len(sd.Repos),
			Detail:    sd.Detail,
		})
	}

	errors := 0
	for _, repo := range repos {
		jr := jsonRepo{
			Name:      repo,
			Pyxis:     presenceStr(src.Pyxis, repo, noPyxis[repo]),
			OBD:       presenceStr(src.OBD, repo, false),
			ImageRefs: presenceStr(src.ImageRefs, repo, false),
			Stage:     presenceStr(src.Stage, repo, false),
			Prod:      presenceStr(src.Prod, repo, false),
		}
		if from, ok := src.ImgRefFrom[repo]; ok {
			jr.From = from
		}
		if delivery, ok := src.OBDDelivery[repo]; ok {
			if deliveryMatch(delivery, repo) {
				jr.Delivery = "ok"
			} else {
				jr.Delivery = "mismatch"
			}
		}
		out.Repos = append(out.Repos, jr)
	}

	for _, iss := range issues {
		if iss.Severity == "error" {
			errors++
		}
		out.Issues = append(out.Issues, jsonIssue{
			Severity:  iss.Severity,
			Repo:      iss.Repo,
			Message:   iss.Message,
			InSources: iss.InSources,
		})
	}

	inSync := len(repos) - errors
	out.Score.InSync = inSync
	out.Score.Total = len(repos)
	if len(repos) > 0 {
		out.Score.Percent = inSync * 100 / len(repos)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

// --- Rendering helpers ---

// sourceIcon returns the display icon for a repo's presence in a source.
func sourceIcon(sd SourceData, repo string, exempt bool) string {
	if !sd.Available || exempt {
		return fmt.Sprintf("%s—%s", cDim, cReset)
	}
	if sd.Repos[repo] {
		return fmt.Sprintf("%s✅%s", cGreen, cReset)
	}
	return fmt.Sprintf("%s❌%s", cRed, cReset)
}

// presenceStr returns a JSON-friendly presence string.
func presenceStr(sd SourceData, repo string, exempt bool) string {
	if !sd.Available {
		return "unavailable"
	}
	if exempt {
		return "exempt"
	}
	if sd.Repos[repo] {
		return "present"
	}
	return "missing"
}

// isAllOK returns true if the repo is present in all available (non-exempt) sources.
func isAllOK(src *Sources, repo string) bool {
	if src.Pyxis.Available && !noPyxis[repo] && !src.Pyxis.Repos[repo] {
		return false
	}
	if src.OBD.Available && !src.OBD.Repos[repo] {
		return false
	}
	if src.ImageRefs.Available && !src.ImageRefs.Repos[repo] {
		return false
	}
	if src.Stage.Available && !src.Stage.Repos[repo] {
		return false
	}
	if src.Prod.Available && !src.Prod.Repos[repo] {
		return false
	}
	return true
}

// colorFromImage formats a quay image URL with colored segments:
// registry (dim), image path (cyan), tag (yellow).
func colorFromImage(ref string) string {
	if ref == "" {
		return ""
	}
	// Split registry/path:tag
	registry := ref
	rest := ""
	if idx := strings.Index(ref, "/"); idx > 0 {
		registry = ref[:idx]
		rest = ref[idx+1:]
	} else {
		return ref
	}
	image := rest
	tag := ""
	if idx := strings.LastIndex(rest, ":"); idx > 0 {
		image = rest[:idx]
		tag = rest[idx+1:]
	}
	return fmt.Sprintf("%s%s/%s%s%s%s%s:%s%s%s%s",
		cDim, registry, cReset,
		cCyan, image, cReset,
		cDim, cReset,
		cYellow, tag, cReset)
}

// repoGroupPrefix extracts a grouping prefix from a repo name for visual separators.
// e.g. "oadp/oadp-velero-plugin-for-aws-rhel9" -> "velero-plugin"
func repoGroupPrefix(repo string) string {
	name := strings.TrimPrefix(repo, "oadp/oadp-")
	if name == repo {
		name = strings.TrimPrefix(repo, "oadp/")
	}
	parts := strings.SplitN(name, "-", 3)
	if len(parts) >= 2 {
		return parts[0] + "-" + parts[1]
	}
	return parts[0]
}
