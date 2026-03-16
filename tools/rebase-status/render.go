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
				// Show commit details when available
				for _, c := range ds.Commits {
					fmt.Fprintf(w, "  %*s    %s%s %s%s\n",
						nameWidth, "",
						cDim, short(c.SHA), c.Message, cReset)
				}
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
				// Show commit details when available
				for _, c := range ds.Commits {
					fmt.Fprintf(w, "      %s%s %s%s\n",
						cDim, short(c.SHA), c.Message, cReset)
				}
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

// RenderMarkdown outputs results as clean Markdown suitable for email or docs.
func RenderMarkdown(w io.Writer, statuses []RepoStatus, checks []Check, branch string) {
	total, ready, errs, warns := scoreboard(statuses)

	fmt.Fprintf(w, "# OADP Rebase Status: %s\n\n", branch)
	fmt.Fprintf(w, "_Generated %s_ | [Rebase Repository](https://github.com/oadp-rebasebot/oadp-rebase)\n\n", time.Now().Format("2006-01-02 15:04 MST"))

	// Score with progress bar
	pct := 0
	if total > 0 {
		pct = ready * 100 / total
	}
	fmt.Fprintf(w, "**%d/%d repos ready (%d%%)**", ready, total, pct)
	if errs > 0 {
		fmt.Fprintf(w, " | %d errors", errs)
	}
	if warns > 0 {
		fmt.Fprintf(w, " | %d warnings", warns)
	}
	fmt.Fprintln(w)

	byWave := groupByWave(statuses)
	waves := sortedWaves(byWave)

	for _, waveNum := range waves {
		repos := byWave[waveNum]
		waveName := waveNameFor(waveNum)

		// Wave header with progress
		waveTotal, waveReady := 0, 0
		for _, r := range repos {
			if r.Spec.Skip {
				continue
			}
			waveTotal++
			hasErr := false
			for _, iss := range r.Issues {
				if iss.Severity == "error" {
					hasErr = true
					break
				}
			}
			if !hasErr {
				waveReady++
			}
		}
		waveIcon := ":white_check_mark:"
		if waveReady < waveTotal {
			waveIcon = ":construction:"
		}
		fmt.Fprintf(w, "\n## %s Wave %d — %s (%d/%d)\n\n", waveIcon, waveNum, waveName, waveReady, waveTotal)

		// Table header
		fmt.Fprint(w, "| Component | Upstream | Rebase CI |")
		for _, chk := range checks {
			fmt.Fprintf(w, " %s |", chk.Header)
		}
		fmt.Fprintln(w)

		// Separator
		fmt.Fprint(w, "| --- | --- | --- |")
		for range checks {
			fmt.Fprint(w, " :---: |")
		}
		fmt.Fprintln(w)

		// Rows
		for _, r := range repos {
			repoLink := mdRepoLink(r.Spec)

			if r.Spec.Skip {
				fmt.Fprintf(w, "| %s | | — |", repoLink)
				for range checks {
					fmt.Fprint(w, " SKIP |")
				}
				fmt.Fprintln(w)
				continue
			}

			upstream := mdUpstreamCell(r.Spec)
			ciLink := mdCILink(r.Spec)
			fmt.Fprintf(w, "| %s | %s | %s |", repoLink, upstream, ciLink)
			for _, chk := range checks {
				result, ok := r.Checks[chk.ID]
				if !ok {
					fmt.Fprint(w, " ? |")
					continue
				}
				switch chk.ID {
				case "image_sync":
					fmt.Fprintf(w, " %s |", mdImgSyncCell(result, r.Images, r.Spec.Repo))
				case "config":
					fmt.Fprintf(w, " %s |", mdRebaseCfgCell(result, r.Spec))
				case "ci_config":
					fmt.Fprintf(w, " %s |", mdProwCfgCell(result, r.Spec))
				case "rebasebot":
					fmt.Fprintf(w, " %s |", mdRebasebotCell(result, r.Spec))
				case "go_version":
					fmt.Fprintf(w, " %s |", mdGoVersionCell(result, r.Spec))
				default:
					fmt.Fprintf(w, " %s |", formatCell(result))
				}
			}
			fmt.Fprintln(w)
		}

		// Details section — collapsible if there are any
		hasAnyDetails := false
		for _, r := range repos {
			if r.Spec.Skip {
				continue
			}
			for _, ds := range r.DepSyncs {
				if !ds.InSync {
					hasAnyDetails = true
					break
				}
			}
			if hasAnyDetails {
				break
			}
			if len(r.Images) > 1 {
				hasAnyDetails = true
			} else {
				for _, img := range r.Images {
					if !img.Exists {
						hasAnyDetails = true
						break
					}
				}
			}
			if hasAnyDetails {
				break
			}
			for _, iss := range r.Issues {
				if mdShouldShowIssue(iss) {
					hasAnyDetails = true
					break
				}
			}
			if hasAnyDetails {
				break
			}
		}

		if hasAnyDetails {
			for _, r := range repos {
				if r.Spec.Skip {
					continue
				}

				hasDetails := false

				// Out-of-sync deps
				for _, ds := range r.DepSyncs {
					if ds.InSync {
						continue
					}
					if !hasDetails {
						fmt.Fprintf(w, "\n**%s**\n", r.Spec.Repo)
						hasDetails = true
					}
					depLink := fmt.Sprintf("https://github.com/%s/%s/tree/%s", ds.Org, ds.Repo, r.Spec.Branch)
					fmt.Fprintf(w, "- :zap: [%s/%s](%s): `%s` → `%s`\n",
						ds.Org, ds.Repo, depLink, short(ds.HaveHash), short(ds.HeadHash))
					for _, c := range ds.Commits {
						commitURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s", ds.Org, ds.Repo, c.SHA)
						fmt.Fprintf(w, "  - [`%s`](%s) %s\n", short(c.SHA), commitURL, c.Message)
					}
				}

				// Images — show all when multiple, or just missing when single
				if len(r.Images) > 1 {
					if !hasDetails {
						fmt.Fprintf(w, "\n**%s**\n", r.Spec.Repo)
						hasDetails = true
					}
					fmt.Fprintf(w, "\n<a id=\"images-%s\"></a>\n", r.Spec.Repo)
					for _, img := range r.Images {
						quayURL := fmt.Sprintf("https://quay.io/repository/%s/%s?tab=tags", img.Namespace, img.Repo)
						if img.Exists {
							fmt.Fprintf(w, "- :white_check_mark: [%s](%s): `:%s` (%s)\n",
								img.Name, quayURL, img.Tag, img.LastModified.Format("2006-01-02"))
						} else {
							fmt.Fprintf(w, "- :x: [%s](%s): missing `:%s`\n", img.Name, quayURL, img.Tag)
						}
					}
				} else {
					for _, img := range r.Images {
						if img.Exists {
							continue
						}
						if !hasDetails {
							fmt.Fprintf(w, "\n**%s**\n", r.Spec.Repo)
							hasDetails = true
						}
						quayURL := fmt.Sprintf("https://quay.io/repository/%s/%s?tab=tags", img.Namespace, img.Repo)
						fmt.Fprintf(w, "- :package: [%s](%s): missing `:%s`\n", img.Name, quayURL, img.Tag)
					}
				}

				// Issues
				for _, iss := range r.Issues {
					if !mdShouldShowIssue(iss) {
						continue
					}
					if !hasDetails {
						fmt.Fprintf(w, "\n**%s**\n", r.Spec.Repo)
						hasDetails = true
					}
					icon := ":x:"
					if iss.Severity == "warning" {
						icon = ":warning:"
					}
					fmt.Fprintf(w, "- %s %s\n", icon, iss.Message)
				}
			}

		}
	}

	// Legend
	fmt.Fprintln(w, "\n---")
	fmt.Fprintln(w, "\n<details>")
	fmt.Fprintln(w, "<summary>Column legend</summary>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "| Column | Description |")
	fmt.Fprintln(w, "| --- | --- |")
	fmt.Fprintln(w, "| **Component** | Repository name (links to rebase PRs) |")
	fmt.Fprintln(w, "| **Upstream** | Upstream repo and branch/tag being rebased from |")
	fmt.Fprintln(w, "| **Rebase CI** | Prow rebasebot job status badge (links to job history) |")
	fmt.Fprintln(w, "| **Rebase Cfg** | Rebase config file exists in oadp-rebase (links to file) |")
	fmt.Fprintln(w, "| **Rebase** | Rebasebot scratch branch exists (links to branch) |")
	fmt.Fprintln(w, "| **Go** | Go version detected in the repo |")
	fmt.Fprintln(w, "| **Prow Cfg** | CI operator config exists in openshift/release (links to dir) |")
	fmt.Fprintln(w, "| **Deps** | Internal OADP dependency sync status |")
	fmt.Fprintln(w, "| **Konflux** | `.konflux/` directory exists on the branch |")
	fmt.Fprintln(w, "| **Image** | Container image tag on Quay (links to tags page) |")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "</details>")
}

// mdShouldShowIssue returns true if the issue should appear in the details section.
func mdShouldShowIssue(iss Issue) bool {
	if strings.Contains(iss.Message, "internal dep(s) out of sync") {
		return false
	}
	if strings.Contains(iss.Message, "images") && strings.Contains(iss.Message, "tag") {
		return false
	}
	if strings.Contains(iss.Message, "image(s) missing tag") {
		return false
	}
	return true
}

// mdUpstreamCell formats the upstream info as a linked markdown table cell.
// Shows just the branch/tag as the label, linking to the full upstream repo+ref.
func mdUpstreamCell(spec RepoSpec) string {
	if spec.Upstream == "" {
		return ""
	}
	u := strings.TrimPrefix(spec.Upstream, "https://github.com/")
	orgRepo := u
	refPart := ""
	if idx := strings.Index(u, ":"); idx != -1 {
		orgRepo = u[:idx]
		refPart = u[idx+1:]
	}
	url := "https://github.com/" + orgRepo
	label := orgRepo
	if refPart != "" {
		url += "/tree/" + refPart
		label = refPart
	}
	return fmt.Sprintf("[%s](%s)", label, url)
}

// mdImgSyncCell formats the image sync cell for markdown using a date linked to Quay.
// For multi-image repos, shows the date with an (existing/total) count linked to the details anchor.
func mdImgSyncCell(result *CheckResult, images []ImageInfo, repoName string) string {
	if result.Status == StatusNA {
		return "—"
	}
	if result.Status == StatusFail {
		if len(images) > 1 {
			existing := 0
			for _, img := range images {
				if img.Exists {
					existing++
				}
			}
			return fmt.Sprintf("%s [(%d/%d)](#images-%s)", result.Status.Icon(), existing, len(images), repoName)
		}
		return result.Status.Icon()
	}
	// Find the oldest image and use it for the date and link
	var oldest *ImageInfo
	for i := range images {
		img := &images[i]
		if img.Exists && (oldest == nil || img.LastModified.Before(oldest.LastModified)) {
			oldest = img
		}
	}
	if oldest == nil {
		return formatCell(result)
	}
	quayURL := fmt.Sprintf("https://quay.io/repository/%s/%s?tab=tags", oldest.Namespace, oldest.Repo)
	dateStr := fmt.Sprintf("[%s](%s)", oldest.LastModified.Format("2006-01-02"), quayURL)
	if len(images) > 1 {
		existing := 0
		for _, img := range images {
			if img.Exists {
				existing++
			}
		}
		return fmt.Sprintf("%s [(%d/%d)](#images-%s)", dateStr, existing, len(images), repoName)
	}
	return dateStr
}

// mdGoVersionCell formats the Go version cell as a link to go.mod on the branch.
func mdGoVersionCell(result *CheckResult, spec RepoSpec) string {
	cell := formatCell(result)
	if result.Status != StatusOK || result.Summary == "" {
		return cell
	}
	goModURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s/go.mod", spec.Org, spec.Repo, spec.Branch)
	return fmt.Sprintf("[%s](%s)", result.Summary, goModURL)
}

// mdRebasebotCell formats the Rebase cell as a link to the rebasebot branch on GitHub.
func mdRebasebotCell(result *CheckResult, spec RepoSpec) string {
	cell := formatCell(result)
	if result.Status != StatusOK {
		return cell
	}
	repo := spec.RebasebotRepo
	branch := spec.RebasebotBranch
	if repo == "" {
		repo = "oadp-rebasebot/" + spec.Repo
		branch = "rebase-bot-" + spec.Branch
	}
	branchURL := fmt.Sprintf("https://github.com/%s/tree/%s", repo, branch)
	return fmt.Sprintf("[%s](%s)", cell, branchURL)
}

// mdRebaseCfgCell formats the Rebase Cfg cell as a link to the config file in oadp-rebase.
func mdRebaseCfgCell(result *CheckResult, spec RepoSpec) string {
	cell := formatCell(result)
	if result.Status != StatusOK {
		return cell
	}
	filename := RebaseConfigFilename(spec.Org, spec.Repo, spec.Branch)
	if filename == "" {
		return cell
	}
	cfgURL := fmt.Sprintf("https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/rebase-configs/%s", filename)
	return fmt.Sprintf("[%s](%s)", cell, cfgURL)
}

// mdProwCfgCell formats the Prow Cfg cell as a link to the ci-operator config dir in openshift/release.
func mdProwCfgCell(result *CheckResult, spec RepoSpec) string {
	cell := formatCell(result)
	if result.Status != StatusOK {
		return cell
	}
	cfgURL := fmt.Sprintf("https://github.com/openshift/release/tree/master/ci-operator/config/%s/%s", spec.Org, spec.Repo)
	return fmt.Sprintf("[%s](%s)", cell, cfgURL)
}

// mdRepoLink returns a Markdown link for the repo name pointing to its rebase PRs.
func mdRepoLink(spec RepoSpec) string {
	prURL := fmt.Sprintf("https://github.com/%s/%s/pulls?q=is%%3Apr+(is%%3Aopen+OR+is%%3Aclosed)+in%%3Atitle+%%22Merge+https%%3A%%2F%%2Fgithub.com%%2F%%22",
		spec.Org, spec.Repo)
	return fmt.Sprintf("[%s](%s)", spec.Repo, prURL)
}

// mdCILink returns a Markdown CI badge for the repo's Prow rebase job.
func mdCILink(spec RepoSpec) string {
	branchDashed := strings.ReplaceAll(spec.Branch, ".", "-")
	jobName := fmt.Sprintf("periodic-ci-openshift-eng-rebasebot-main-%s-%s-%s",
		spec.Org, spec.Repo, branchDashed)
	badgeURL := fmt.Sprintf("https://prow.ci.openshift.org/badge.svg?jobs=%s", jobName)
	historyURL := fmt.Sprintf("https://prow.ci.openshift.org/job-history/gs/origin-ci-test/logs/%s", jobName)
	return fmt.Sprintf("[![%s](%s)](%s)", spec.Branch, badgeURL, historyURL)
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
				fmt.Fprintf(w, "{\"module\": \"%s\", \"org\": \"%s\", \"repo\": \"%s\", \"have\": \"%s\", \"head\": \"%s\", \"in_sync\": %t",
					ds.Module, ds.Org, ds.Repo, ds.HaveHash, ds.HeadHash, ds.InSync)
				if len(ds.Commits) > 0 {
					fmt.Fprint(w, ", \"commits\": [")
					for ci, cm := range ds.Commits {
						if ci > 0 {
							fmt.Fprint(w, ", ")
						}
						// Escape quotes in commit messages for valid JSON
						escapedMsg := strings.ReplaceAll(cm.Message, "\\", "\\\\")
						escapedMsg = strings.ReplaceAll(escapedMsg, "\"", "\\\"")
						fmt.Fprintf(w, "{\"sha\": \"%s\", \"message\": \"%s\"}", cm.SHA, escapedMsg)
					}
					fmt.Fprint(w, "]")
				}
				fmt.Fprint(w, "}")
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
