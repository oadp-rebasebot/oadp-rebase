package main

import (
	"fmt"
	"io"
	"os"
	"sort"
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

// RenderTable prints the status report as a formatted terminal table.
func RenderTable(w io.Writer, statuses []RepoStatus, checks []Check, branch string) {
	// Header
	fmt.Fprintf(w, "\n%s%sOADP Rebase Status: %s%s\n", cBold, cCyan, branch, cReset)
	fmt.Fprintf(w, "%s%s%s\n", cDim, time.Now().Format("2006-01-02 15:04 MST"), cReset)
	fmt.Fprintln(w, strings.Repeat("═", 70))

	// Group by wave
	byWave := groupByWave(statuses)
	waves := sortedWaves(byWave)

	var allIssues []Issue

	for _, waveNum := range waves {
		repos := byWave[waveNum]
		waveName := waveNameFor(waveNum)

		fmt.Fprintf(w, "\n%s%sWave %d — %s%s\n", cBold, cCyan, waveNum, waveName, cReset)

		// Compute repo name column width
		nameWidth := 4
		for _, r := range repos {
			n := len(r.Spec.FullName())
			if n > nameWidth {
				nameWidth = n
			}
		}

		// Print column headers
		fmt.Fprintf(w, "  %-*s", nameWidth, "Repo")
		for _, chk := range checks {
			fmt.Fprintf(w, "  %-7s", chk.Header)
		}
		fmt.Fprintln(w)

		// Print separator
		fmt.Fprintf(w, "  %s%s", cDim, strings.Repeat("─", nameWidth))
		for range checks {
			fmt.Fprintf(w, "  %s", strings.Repeat("─", 7))
		}
		fmt.Fprintf(w, "%s\n", cReset)

		// Print rows
		for _, r := range repos {
			if r.Spec.Skip {
				fmt.Fprintf(w, "  %s%-*s  SKIP%s\n", cDim, nameWidth, r.Spec.FullName(), cReset)
				continue
			}

			// Repo name + check columns
			fmt.Fprintf(w, "  %-*s", nameWidth, r.Spec.FullName())
			for _, chk := range checks {
				result, ok := r.Checks[chk.ID]
				if !ok {
					fmt.Fprintf(w, "  %-7s", "?")
					continue
				}
				cell := formatCell(result)
				fmt.Fprintf(w, "  %s", colorCell(cell, result.Status, 7))
			}
			fmt.Fprintln(w)

			// Upstream info as a sub-line
			upstream := formatUpstream(r.Spec)
			if upstream != "" {
				fmt.Fprintf(w, "  %*s  %s→ %s%s\n", nameWidth, "", cDim, upstream, cReset)
			}

			// Dep sync sub-lines (only show out-of-sync ones to reduce noise)
			for _, ds := range r.DepSyncs {
				if ds.InSync {
					continue
				}
				fmt.Fprintf(w, "  %*s  %s⚡ %s/%s: %s%s → %s%s\n",
					nameWidth, "",
					cRed, ds.Org, ds.Repo,
					cYellow, short(ds.HaveHash), short(ds.HeadHash),
					cReset)
			}

			// Image sub-lines (show details when >1 image or any missing)
			if len(r.Images) > 1 || (len(r.Images) > 0 && !r.Images[0].Exists) {
				for _, img := range r.Images {
					if img.Exists {
						fmt.Fprintf(w, "  %*s  %s📦 %s: %s%s\n",
							nameWidth, "",
							cGreen, img.Name, FormatAge(img.LastModified), cReset)
					} else {
						fmt.Fprintf(w, "  %*s  %s📦 %s: missing :%s%s\n",
							nameWidth, "",
							cRed, img.Name, img.Tag, cReset)
					}
				}
			}

			allIssues = append(allIssues, r.Issues...)
		}
	}

	// Issues summary
	fmt.Fprintln(w)
	if len(allIssues) == 0 {
		fmt.Fprintf(w, "%s%sNo issues found. All checks passed!%s ✅\n", cBold, cGreen, cReset)
	} else {
		errors := 0
		warnings := 0
		for _, iss := range allIssues {
			if iss.Severity == "error" {
				errors++
			} else {
				warnings++
			}
		}
		// Filter out issues already shown inline
		var filtered []Issue
		for _, iss := range allIssues {
			if strings.Contains(iss.Message, "internal dep(s) out of sync") {
				continue
			}
			if strings.Contains(iss.Message, "images") && strings.Contains(iss.Message, "tag") {
				continue
			}
			if strings.Contains(iss.Message, "image(s) missing tag") {
				continue
			}
			filtered = append(filtered, iss)
		}

		fmt.Fprintf(w, "%sIssues (%s%d errors%s, %s%d warnings%s):%s\n",
			cBold, cRed, errors, cReset+cBold, cYellow, warnings, cReset+cBold, cReset)
		for _, iss := range filtered {
			icon := "❌"
			color := cRed
			if iss.Severity == "warning" {
				icon = "⚠️ "
				color = cYellow
			}
			fmt.Fprintf(w, "  %s  %-40s  %s%s%s\n", icon, iss.Repo, color, iss.Message, cReset)
		}
	}

	// Score
	total := 0
	ready := 0
	for _, r := range statuses {
		if r.Spec.Skip {
			continue
		}
		total++
		hasError := false
		for _, iss := range r.Issues {
			if iss.Severity == "error" {
				hasError = true
				break
			}
		}
		if !hasError {
			ready++
		}
	}
	scoreColor := cRed
	if ready == total {
		scoreColor = cGreen
	} else if ready*2 >= total {
		scoreColor = cYellow
	}
	fmt.Fprintf(w, "\n%sScore: %s%d/%d repos ready", cBold, scoreColor, ready, total)
	if total > 0 {
		fmt.Fprintf(w, " (%d%%)", ready*100/total)
	}
	fmt.Fprintf(w, "%s\n", cReset)
}

// colorCell returns a fixed-width colored string for a table cell.
func colorCell(text string, status Status, width int) string {
	color := ""
	switch status {
	case StatusOK:
		color = cGreen
	case StatusFail:
		color = cRed
	case StatusWarn:
		color = cYellow
	case StatusNA, StatusSkip:
		color = cDim
	}
	// Pad to width, then wrap in color
	padded := fmt.Sprintf("%-*s", width, text)
	return color + padded + cReset
}

// RenderText prints a card-style text view — one block per repo, no table grid.
func RenderText(w io.Writer, statuses []RepoStatus, checks []Check, branch string) {
	fmt.Fprintf(w, "\n%s%sOADP Rebase Status: %s%s\n", cBold, cCyan, branch, cReset)
	fmt.Fprintf(w, "%s%s%s\n\n", cDim, time.Now().Format("2006-01-02 15:04 MST"), cReset)

	total, ready, errs, warns := scoreboard(statuses)
	scoreColor := cRed
	if ready == total {
		scoreColor = cGreen
	} else if ready*2 >= total {
		scoreColor = cYellow
	}
	fmt.Fprintf(w, "  %s%s%d/%d repos ready%s", cBold, scoreColor, ready, total, cReset)
	if errs > 0 {
		fmt.Fprintf(w, "  %s%d errors%s", cRed, errs, cReset)
	}
	if warns > 0 {
		fmt.Fprintf(w, "  %s%d warnings%s", cYellow, warns, cReset)
	}
	fmt.Fprintln(w)

	byWave := groupByWave(statuses)
	waves := sortedWaves(byWave)

	for _, waveNum := range waves {
		repos := byWave[waveNum]
		waveName := waveNameFor(waveNum)

		fmt.Fprintf(w, "\n%s%s╾── Wave %d: %s ──╼%s\n", cBold, cCyan, waveNum, waveName, cReset)

		for _, r := range repos {
			name := r.Spec.FullName()

			if r.Spec.Skip {
				fmt.Fprintf(w, "  %s%-44s SKIP%s\n", cDim, name, cReset)
				continue
			}

			hasError := false
			for _, iss := range r.Issues {
				if iss.Severity == "error" {
					hasError = true
					break
				}
			}
			indicator := cGreen + "●" + cReset
			if hasError {
				indicator = cRed + "●" + cReset
			}
			fmt.Fprintf(w, "\n  %s %s%s%s\n", indicator, cBold, name, cReset)

			upstream := formatUpstream(r.Spec)
			if upstream != "" {
				fmt.Fprintf(w, "    %s↑ %s%s\n", cDim, upstream, cReset)
			}

			// Check badges on one line
			fmt.Fprint(w, "    ")
			for _, chk := range checks {
				result, ok := r.Checks[chk.ID]
				if !ok {
					fmt.Fprintf(w, "%s%s%s:? ", cDim, chk.Header, cReset)
					continue
				}
				cell := chk.Header
				if result.Summary != "" {
					cell += ":" + result.Summary
				}
				color := ""
				switch result.Status {
				case StatusOK:
					color = cGreen
				case StatusFail:
					color = cRed
				case StatusWarn:
					color = cYellow
				default:
					color = cDim
				}
				fmt.Fprintf(w, "%s%s%s ", color, cell, cReset)
			}
			fmt.Fprintln(w)

			// Dep sync sub-lines
			for _, ds := range r.DepSyncs {
				if ds.InSync {
					continue
				}
				fmt.Fprintf(w, "    %s⚡ %s/%s: %s%s → %s%s\n",
					cRed, ds.Org, ds.Repo,
					cYellow, short(ds.HaveHash), short(ds.HeadHash), cReset)
			}

			// Image sub-lines
			if len(r.Images) > 1 || (len(r.Images) > 0 && !r.Images[0].Exists) {
				for _, img := range r.Images {
					if img.Exists {
						fmt.Fprintf(w, "    %s📦 %s: %s%s\n",
							cGreen, img.Name, FormatAge(img.LastModified), cReset)
					} else {
						fmt.Fprintf(w, "    %s📦 %s: missing :%s%s\n",
							cRed, img.Name, img.Tag, cReset)
					}
				}
			}

			for _, iss := range r.Issues {
				if strings.Contains(iss.Message, "internal dep(s) out of sync") {
					continue
				}
				if strings.Contains(iss.Message, "images") && strings.Contains(iss.Message, "tag") {
					continue
				}
				if iss.Severity == "error" {
					fmt.Fprintf(w, "    %s%s%s\n", cRed, iss.Message, cReset)
				}
			}
		}
	}

	fmt.Fprintln(w)
}

// RenderJSON outputs results as JSON (for scripting).
func RenderJSON(w io.Writer, statuses []RepoStatus) {
	fmt.Fprintln(w, "[")
	for i, r := range statuses {
		fmt.Fprintf(w, "  {\"repo\": \"%s\", \"branch\": \"%s\", \"wave\": %d, \"skip\": %t, \"upstream\": \"%s\", \"checks\": {",
			r.Spec.FullName(), r.Spec.Branch, r.Spec.Wave, r.Spec.Skip,
			strings.ReplaceAll(formatUpstream(r.Spec), "\"", "\\\""))
		j := 0
		for id, result := range r.Checks {
			if j > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "\"%s\": {\"status\": \"%s\", \"summary\": \"%s\"}",
				id, statusName(result.Status), result.Summary)
			j++
		}
		fmt.Fprint(w, "}")
		if len(r.DepSyncs) > 0 {
			fmt.Fprint(w, ", \"dep_syncs\": [")
			for k, ds := range r.DepSyncs {
				if k > 0 {
					fmt.Fprint(w, ", ")
				}
				fmt.Fprintf(w, "{\"module\": \"%s\", \"org\": \"%s\", \"repo\": \"%s\", \"have\": \"%s\", \"head\": \"%s\", \"in_sync\": %t}",
					ds.Module, ds.Org, ds.Repo, ds.HaveHash, ds.HeadHash, ds.InSync)
			}
			fmt.Fprint(w, "]")
		}
		if len(r.Issues) > 0 {
			fmt.Fprint(w, ", \"issues\": [")
			for k, iss := range r.Issues {
				if k > 0 {
					fmt.Fprint(w, ", ")
				}
				fmt.Fprintf(w, "{\"severity\": \"%s\", \"message\": \"%s\"}", iss.Severity, iss.Message)
			}
			fmt.Fprint(w, "]")
		}
		fmt.Fprint(w, "}")
		if i < len(statuses)-1 {
			fmt.Fprintln(w, ",")
		} else {
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintln(w, "]")
}

// ---------- Formatting helpers ----------

func formatCell(r *CheckResult) string {
	if r.Summary != "" {
		return r.Summary
	}
	return r.Status.Icon()
}

func formatUpstream(spec RepoSpec) string {
	if spec.Upstream == "" {
		return ""
	}
	u := strings.TrimPrefix(spec.Upstream, "https://github.com/")
	if idx := strings.Index(u, ":"); idx != -1 {
		return u[:idx] + " @ " + u[idx+1:]
	}
	return u
}

func waveNameFor(n int) string {
	for _, w := range wavesMeta {
		if w.Number == n {
			return w.Name
		}
	}
	return "unknown"
}

func groupByWave(statuses []RepoStatus) map[int][]RepoStatus {
	m := make(map[int][]RepoStatus)
	for _, s := range statuses {
		w := s.Spec.Wave
		if w == 0 {
			w = 99
		}
		m[w] = append(m[w], s)
	}
	return m
}

func sortedWaves(m map[int][]RepoStatus) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func scoreboard(statuses []RepoStatus) (total, ready, errors, warnings int) {
	for _, r := range statuses {
		if r.Spec.Skip {
			continue
		}
		total++
		hasError := false
		for _, iss := range r.Issues {
			if iss.Severity == "error" {
				errors++
				hasError = true
			} else if iss.Severity == "warning" {
				warnings++
			}
		}
		if !hasError {
			ready++
		}
	}
	return
}

func statusName(s Status) string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusFail:
		return "fail"
	case StatusWarn:
		return "warn"
	case StatusSkip:
		return "skip"
	case StatusNA:
		return "na"
	default:
		return "unknown"
	}
}
