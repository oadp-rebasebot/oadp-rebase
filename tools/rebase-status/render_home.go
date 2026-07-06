package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const wikiWorkflowURL = "https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/rebase-status-wiki.yaml"

type openPREntry struct {
	Repo string
	Wave int
	PR   *OpenPRInfo
}

type waveStatus struct {
	Number int
	Name   string
	Ready  int
	Total  int
}

func computeWaveStatuses(statuses []RepoStatus) []waveStatus {
	byWave := groupByWave(statuses)
	waves := sortedWaves(byWave)
	var result []waveStatus
	for _, wn := range waves {
		wt, wr := 0, 0
		for _, r := range byWave[wn] {
			if r.Spec.Skip {
				continue
			}
			wt++
			hasErr := false
			for _, iss := range r.Issues {
				if iss.Severity == "error" {
					hasErr = true
					break
				}
			}
			if !hasErr {
				wr++
			}
		}
		result = append(result, waveStatus{wn, waveNameFor(wn), wr, wt})
	}
	return result
}

func collectOpenPRs(statuses []RepoStatus) []openPREntry {
	byWave := groupByWave(statuses)
	waves := sortedWaves(byWave)
	var result []openPREntry
	for _, wn := range waves {
		for _, r := range byWave[wn] {
			if r.OpenPR != nil {
				result = append(result, openPREntry{r.Spec.Repo, wn, r.OpenPR})
			}
		}
	}
	return result
}

func latestOpenPR(prs []openPREntry) *openPREntry {
	if len(prs) == 0 {
		return nil
	}
	best := &prs[0]
	for i := 1; i < len(prs); i++ {
		if prs[i].PR.CreatedAt.After(best.PR.CreatedAt) {
			best = &prs[i]
		}
	}
	return best
}

func daysAgo(t time.Time) int {
	return int(time.Since(t).Hours() / 24)
}

func formatDaysAgo(days int) string {
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "1 day ago"
	default:
		return fmt.Sprintf("%d days ago", days)
	}
}

func homeScoreLine(total, ready, errs, warns int) string {
	s := fmt.Sprintf("%d/%d repos ready", ready, total)
	if total > 0 {
		s += fmt.Sprintf(" (%d%%)", ready*100/total)
	}
	if errs > 0 {
		s += fmt.Sprintf(" | %d error(s)", errs)
	}
	if warns > 0 {
		s += fmt.Sprintf(" | %d warning(s)", warns)
	}
	return s
}

// githubAnchor converts a heading into a GitHub wiki anchor.
// e.g. ":construction: Wave 4 — Downstream Controllers (3/4)"
//
//	→ "construction-wave-4--downstream-controllers-34"
func githubAnchor(heading string) string {
	s := strings.ToLower(heading)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RenderHome generates the wiki Home.md from multiple branch results.
func RenderHome(w io.Writer, branches []BranchResult) {
	fmt.Fprintln(w, "# OADP Rebase Status")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "[**Dependency Graph**](https://oadp-rebasebot.github.io/oadp-rebase/rebase-dag/) | _Updated automatically by [rebase-status-wiki workflow](%s)_\n", wikiWorkflowURL)
	fmt.Fprintln(w)

	for _, br := range branches {
		page := "Rebase-Status-" + br.Branch
		total, ready, errs, warns := scoreboard(br.Statuses)
		scoreLine := homeScoreLine(total, ready, errs, warns)
		waveStats := computeWaveStatuses(br.Statuses)
		openPRs := collectOpenPRs(br.Statuses)
		latest := latestOpenPR(openPRs)

		// Branch header + score
		fmt.Fprintf(w, "## [%s](%s)\n\n", br.Branch, page)
		fmt.Fprintln(w, scoreLine)
		fmt.Fprintln(w)

		// Velero tag alignment summary
		if br.VeleroTagAlign != nil {
			vtaHome := br.VeleroTagAlign
			aligned := 0
			for _, r := range vtaHome.Repos {
				if r.Aligned {
					aligned++
				}
			}
			if vtaHome.AllAligned {
				fmt.Fprintf(w, ":white_check_mark: Velero `%s` — all %d repos aligned\n\n", vtaHome.VeleroTag, len(vtaHome.Repos))
			} else {
				fmt.Fprintf(w, ":construction: Velero `%s` — %d/%d repos aligned ([details](%s#velero-tag-alignment))\n\n",
					vtaHome.VeleroTag, aligned, len(vtaHome.Repos), page)
			}
		}

		// CVE PR summary
		if len(br.CVEPRs) > 0 {
			page := "Rebase-Status-" + br.Branch
			fmt.Fprintf(w, ":shield: %d open CVE fix PR(s) — [details](%s#shield-open-cve-fix-prs-%d)\n\n",
				len(br.CVEPRs), page, len(br.CVEPRs))
		} else {
			fmt.Fprintln(w, ":white_check_mark: No open CVE fix PRs")
			fmt.Fprintln(w)
		}

		// TODO if no configs exist for this branch
		hasConfigs := false
		for _, s := range br.Statuses {
			if s.Spec.HasConfig {
				hasConfigs = true
				break
			}
		}
		if !hasConfigs && total > 0 {
			fmt.Fprintf(w, "> **TODO:** Rebase configs are yet to be added for %s\n\n", br.Branch)
		}

		// Latest PR with days-ago
		if latest != nil {
			age := formatDaysAgo(daysAgo(latest.PR.CreatedAt))
			fmt.Fprintf(w, "> Latest PR: [%s #%d](%s) Wave %d · **%s**\n\n",
				latest.Repo, latest.PR.Number, latest.PR.URL, latest.Wave, age)
		}

		// Waves needing attention
		var attentionWaves []waveStatus
		for _, ws := range waveStats {
			if ws.Ready < ws.Total {
				attentionWaves = append(attentionWaves, ws)
				title := fmt.Sprintf("Wave %d — %s (%d/%d)", ws.Number, ws.Name, ws.Ready, ws.Total)
				anchor := githubAnchor(":construction: " + title)
				fmt.Fprintf(w, "- :construction: [%s](%s#%s)\n", title, page, anchor)
			}
		}

		// Open PRs list
		if len(openPRs) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "**Open rebase PRs:**")
			fmt.Fprintln(w)
			for _, p := range openPRs {
				fmt.Fprintf(w, "- [%s #%d](%s) — Wave %d (%s)\n",
					p.Repo, p.PR.Number, p.PR.URL, p.Wave,
					p.PR.CreatedAt.Format("2006-01-02"))
			}
		}

		// Slack copy snippet
		wikiURL := fmt.Sprintf("https://github.com/oadp-rebasebot/oadp-rebase/wiki/%s", page)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "<details>")
		fmt.Fprintln(w, "<summary>Copy</summary>")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "```")
		fmt.Fprintf(w, "*OADP Rebase Status: %s*\n", br.Branch)
		fmt.Fprintln(w, scoreLine)

		if br.VeleroTagAlign != nil {
			vtaCopy := br.VeleroTagAlign
			alignedCopy := 0
			for _, r := range vtaCopy.Repos {
				if r.Aligned {
					alignedCopy++
				}
			}
			if vtaCopy.AllAligned {
				fmt.Fprintf(w, ":white_check_mark: Velero %s — all %d repos aligned\n", vtaCopy.VeleroTag, len(vtaCopy.Repos))
			} else {
				fmt.Fprintf(w, ":construction: Velero %s — %d/%d repos aligned\n", vtaCopy.VeleroTag, alignedCopy, len(vtaCopy.Repos))
			}
		}

		if len(br.CVEPRs) > 0 {
			fmt.Fprintf(w, ":shield: %d open CVE fix PR(s)\n", len(br.CVEPRs))
			for _, pr := range br.CVEPRs {
				fmt.Fprintf(w, "  • %s/%s #%d — %s\n    %s\n", pr.Org, pr.Repo, pr.Number, pr.Title, pr.URL)
			}
		} else {
			fmt.Fprintln(w, ":white_check_mark: No open CVE fix PRs")
		}

		if len(attentionWaves) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, ":construction: Waves needing attention:")
			for _, ws := range attentionWaves {
				fmt.Fprintf(w, "  • Wave %d — %s (%d/%d)\n", ws.Number, ws.Name, ws.Ready, ws.Total)
			}
		}

		if len(openPRs) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, ":memo: Open rebase PRs:")
			for _, p := range openPRs {
				fmt.Fprintf(w, "  • %s #%d — Wave %d (%s)\n    %s\n",
					p.Repo, p.PR.Number, p.Wave,
					p.PR.CreatedAt.Format("2006-01-02"), p.PR.URL)
			}
		}

		fmt.Fprintln(w)
		fmt.Fprintf(w, "Full status: %s\n", wikiURL)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "</details>")
		fmt.Fprintln(w)
	}
}
