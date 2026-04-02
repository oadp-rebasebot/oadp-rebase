package main

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// queueSortMode determines how repos are ordered in the queue tab.
type queueSortMode int

const (
	queueSortName         queueSortMode = iota // alphabetical
	queueSortPRCount                           // most PRs first
	queueSortBlocked                           // most blocked first
	queueSortTideErrLoop                       // repos with tideErrLoopBlocker PRs first
)

func (s queueSortMode) String() string {
	switch s {
	case queueSortName:
		return "name"
	case queueSortPRCount:
		return "PR count"
	case queueSortBlocked:
		return "blocked"
	case queueSortTideErrLoop:
		return "tideErrLoop"
	default:
		return ""
	}
}

type queueModel struct {
	queues       map[string]*MergeQueueReport
	repoOrder    []string        // repos with queue results
	allRepos     []string        // all repos from audit (for showing unchecked repos)
	collapsed    map[string]bool // repo -> collapsed
	prCollapsed  map[string]bool // "repo#number" -> PR details collapsed
	allQueueRows []queueRow      // unfiltered rows from buildRows
	rows         []queueRow
	filterQuery  string
	cursor       int
	scrollOff    int
	loading      map[string]bool // repo -> loading state
	sortMode     queueSortMode
	ghAvailable  bool
	hideHealthy  bool // when true, repos with no blockers are hidden (default true)
}

type queueRow struct {
	kind     rowKind
	repoName string
	prKey    string // "repo#number" for PR-level collapse tracking
	text     string
	severity Severity
	url      string // if set, right-arrow opens this URL in browser
}

func newQueueModel() queueModel {
	m := queueModel{
		queues:      make(map[string]*MergeQueueReport),
		collapsed:   make(map[string]bool),
		prCollapsed: make(map[string]bool),
		loading:     make(map[string]bool),
		hideHealthy: true, // hide repos with no blockers by default
	}
	m.buildRows()
	return m
}

// repoIsHealthy returns true if a repo has no actionable blockers.
// A healthy repo has either no PRs or all PRs are ready (no blockers).
func (m *queueModel) repoIsHealthy(repo string) bool {
	report, ok := m.queues[repo]
	if !ok {
		return false // no data yet, not considered healthy
	}
	for _, pr := range report.PRs {
		if pr.HasBlockers {
			return false
		}
	}
	return true
}

// SetQueue adds or updates a repo's queue data and rebuilds rows.
func (m *queueModel) SetQueue(repo string, report *MergeQueueReport) {
	m.queues[repo] = report
	delete(m.loading, repo)

	found := false
	for _, r := range m.repoOrder {
		if r == repo {
			found = true
			break
		}
	}
	if !found {
		m.repoOrder = append(m.repoOrder, repo)
	}

	m.buildRows()
}

func (m *queueModel) sortedRepos() []string {
	repos := make([]string, len(m.repoOrder))
	copy(repos, m.repoOrder)

	switch m.sortMode {
	case queueSortPRCount:
		sort.Slice(repos, func(i, j int) bool {
			ri, rj := m.queues[repos[i]], m.queues[repos[j]]
			ci, cj := 0, 0
			if ri != nil {
				ci = len(ri.PRs)
			}
			if rj != nil {
				cj = len(rj.PRs)
			}
			if ci != cj {
				return ci > cj // descending
			}
			return repos[i] < repos[j]
		})
	case queueSortBlocked:
		sort.Slice(repos, func(i, j int) bool {
			ri, rj := m.queues[repos[i]], m.queues[repos[j]]
			bi, bj := 0, 0
			if ri != nil {
				for _, pr := range ri.PRs {
					if pr.HasBlockers {
						bi++
					}
				}
			}
			if rj != nil {
				for _, pr := range rj.PRs {
					if pr.HasBlockers {
						bj++
					}
				}
			}
			if bi != bj {
				return bi > bj // most blocked first
			}
			return repos[i] < repos[j]
		})
	case queueSortTideErrLoop:
		sort.Slice(repos, func(i, j int) bool {
			ri, rj := m.queues[repos[i]], m.queues[repos[j]]
			ti, tj := 0, 0
			if ri != nil {
				for _, pr := range ri.PRs {
					if pr.ReviewBlocked && ri.EnforceAdmins && len(pr.LabelBlockers) == 0 {
						ti++
					}
				}
			}
			if rj != nil {
				for _, pr := range rj.PRs {
					if pr.ReviewBlocked && rj.EnforceAdmins && len(pr.LabelBlockers) == 0 {
						tj++
					}
				}
			}
			// Repos with tideErrLoopBlocker PRs first.
			hasI, hasJ := ti > 0, tj > 0
			if hasI != hasJ {
				return hasI
			}
			// Then by count descending.
			if ti != tj {
				return ti > tj
			}
			return repos[i] < repos[j]
		})
	default: // queueSortName
		sort.Strings(repos)
	}
	return repos
}

func (m *queueModel) buildRows() {
	m.rows = nil

	// Action button: check all queues.
	if m.ghAvailable {
		m.rows = append(m.rows, queueRow{
			kind:     rowAction,
			text:     "  [M] Check All Merge Queues",
			severity: SeverityInfo,
		})
	} else {
		m.rows = append(m.rows, queueRow{
			kind:     rowRepoDetail,
			text:     "  ⚠ Merge queue requires gh CLI — install: https://cli.github.com/",
			severity: SeverityWarning,
		})
	}

	// Show repos currently being fetched (not yet in queues).
	for repo := range m.loading {
		if _, hasResult := m.queues[repo]; hasResult {
			continue // already have results, will show below
		}
		m.rows = append(m.rows, queueRow{
			kind:     rowGroupHeader,
			repoName: repo,
			text:     fmt.Sprintf("◉ %s (loading...)", repo),
			severity: SeverityInfo,
		})
	}

	// Count healthy repos to show summary when hidden.
	healthyCount := 0
	if m.hideHealthy {
		for _, repo := range m.sortedRepos() {
			if m.repoIsHealthy(repo) {
				healthyCount++
			}
		}
	}

	for _, repo := range m.sortedRepos() {
		report, ok := m.queues[repo]
		if !ok {
			continue
		}

		// Skip healthy repos when hideHealthy is enabled.
		if m.hideHealthy && m.repoIsHealthy(repo) {
			continue
		}

		// Show error report as a warning row instead of normal content.
		if report.Error != "" {
			m.rows = append(m.rows, queueRow{
				kind:     rowGroupHeader,
				repoName: repo,
				text:     fmt.Sprintf("[m] %s %s", IconWarn, repo),
				severity: SeverityWarning,
			})
			m.rows = append(m.rows, queueRow{
				kind:     rowRepoDetail,
				repoName: repo,
				text:     fmt.Sprintf("    %s %s", IconWarn, report.Error),
				severity: SeverityWarning,
			})
			continue
		}

		// Repo header — collapsible.
		arrow := IconExpanded
		if m.collapsed[repo] {
			arrow = IconCollapsed
		}

		// Compute a quick status summary for the collapsed header.
		readyCount := 0
		blockedCount := 0
		tideErrLoopCount := 0
		for _, pr := range report.PRs {
			if pr.HasBlockers {
				blockedCount++
				// tideErrLoopBlocker: has labels but fails branch protection review count
				// Only when enforce_admins=true — otherwise Tide merges on labels alone
				if pr.ReviewBlocked && report.EnforceAdmins && len(pr.LabelBlockers) == 0 {
					tideErrLoopCount++
				}
			} else {
				readyCount++
			}
		}
		var statusParts []string
		if len(report.PRs) == 0 {
			statusParts = append(statusParts, "no PRs")
		} else {
			if readyCount > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d ready", readyCount))
			}
			if tideErrLoopCount > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d tideErrLoopBlocker", tideErrLoopCount))
			}
			if blockedCount-tideErrLoopCount > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d blocked", blockedCount-tideErrLoopCount))
			}
		}
		statusHint := strings.Join(statusParts, ", ")

		headerSev := SeverityOK
		if tideErrLoopCount > 0 {
			headerSev = SeverityIssue
		} else if blockedCount > 0 {
			headerSev = SeverityWarning
		}

		headerText := fmt.Sprintf("[m] %s %s (%d PRs — %s)", arrow, repo, len(report.PRs), statusHint)
		m.rows = append(m.rows, queueRow{
			kind:     rowGroupHeader,
			repoName: repo,
			text:     headerText,
			severity: headerSev,
		})

		if m.collapsed[repo] {
			continue
		}

		// Sub-header with review requirements.
		subHeader := fmt.Sprintf("  Required reviews: %d │ enforce_admins: %v",
			report.RequiredReviews, report.EnforceAdmins)
		subSev := SeverityOK
		if report.EnforceAdmins && report.RequiredReviews > 0 {
			subHeader += "  (GitHub branch protection enforced — tideErrLoopBlocker risk, see prow#134)"
			subSev = SeverityInfo
		} else if !report.EnforceAdmins {
			subHeader += "  (Tide merges on labels alone — no branch protection)"
		}
		m.rows = append(m.rows, queueRow{
			kind:     rowRepoDetail,
			repoName: repo,
			text:     subHeader,
			severity: subSev,
		})

		// Tide URL.
		if report.TideURL != "" {
			m.rows = append(m.rows, queueRow{
				kind:     rowRepoDetail,
				repoName: repo,
				text:     "  Tide: " + report.TideURL,
				severity: SeverityOK,
				url:      report.TideURL,
			})
		}

		if len(report.PRs) == 0 {
			m.rows = append(m.rows, queueRow{
				kind:     rowRepoDetail,
				repoName: repo,
				text:     "    No PRs with approved+lgtm",
				severity: SeverityInfo,
			})
			continue
		}

		// Collect tideErrLoopBlocker PRs for this repo (blocks entire Tide queue, not just same branch).
		var tideErrLoopBlockerPRs []int
		for _, pr := range report.PRs {
			if pr.ReviewBlocked && report.EnforceAdmins && len(pr.LabelBlockers) == 0 {
				tideErrLoopBlockerPRs = append(tideErrLoopBlockerPRs, pr.Number)
			}
		}

		// Sort PRs: tideErrLoopBlocker blockers first, then other blocked, then ready.
		sortedPRs := make([]PRStatus, len(report.PRs))
		copy(sortedPRs, report.PRs)
		sort.SliceStable(sortedPRs, func(i, j int) bool {
			pi, pj := sortedPRs[i], sortedPRs[j]
			iIs134 := pi.ReviewBlocked && report.EnforceAdmins && len(pi.LabelBlockers) == 0
			jIs134 := pj.ReviewBlocked && report.EnforceAdmins && len(pj.LabelBlockers) == 0
			// tideErrLoopBlocker blockers first
			if iIs134 != jIs134 {
				return iIs134
			}
			// then other blocked PRs
			if pi.HasBlockers != pj.HasBlockers {
				return pi.HasBlockers
			}
			return false
		})

		for _, pr := range sortedPRs {
			prKey := fmt.Sprintf("%s#%d", repo, pr.Number)

			// tideErrLoopBlocker queue blocker: PR has approved+lgtm (Tide will try to merge)
			// AND enforce_admins=true AND insufficient GitHub review approvals.
			// Tide picks the PR, GitHub rejects (branch protection), Tide retries = error loop.
			// Only possible when enforce_admins=true — without it, Tide merges on labels alone.
			isTideErrLoopBlocker := pr.ReviewBlocked && report.EnforceAdmins && len(pr.LabelBlockers) == 0

			// Check if other PRs in the repo are tideErrLoopBlockers (affects this PR even if ready).
			hasRepoTideErrBlockers := false
			if !pr.HasBlockers {
				for _, blockerNum := range tideErrLoopBlockerPRs {
					if blockerNum != pr.Number {
						hasRepoTideErrBlockers = true
						break
					}
				}
			}

			icon := "✗"
			prSev := SeverityWarning // regular blocked PR
			if isTideErrLoopBlocker {
				icon = "⚠"
				prSev = SeverityIssue // most severe — blocks entire queue
			} else if !pr.HasBlockers && hasRepoTideErrBlockers {
				icon = "⚠"
				prSev = SeverityWarning // ready but Tide queue may be stuck
			} else if !pr.HasBlockers {
				icon = "✓"
				prSev = SeverityOK
			}

			totalChecks := len(pr.Checks.Failing) + len(pr.Checks.Errored) + len(pr.Checks.Pending) + len(pr.Checks.InProgress)
			failCount := len(pr.Checks.Failing) + len(pr.Checks.Errored)

			var summaryParts []string
			if isTideErrLoopBlocker {
				summaryParts = append(summaryParts, fmt.Sprintf("⚠ BLOCKS QUEUE — tideErrLoopBlocker: reviews %d/%d", pr.ApprovalCount, pr.RequiredReviews))
			} else if pr.ReviewBlocked {
				summaryParts = append(summaryParts, fmt.Sprintf("reviews %d/%d", pr.ApprovalCount, pr.RequiredReviews))
			}
			if failCount > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("%d/%d checks failed", failCount, totalChecks))
			}
			if len(pr.LabelBlockers) > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("%d label blockers", len(pr.LabelBlockers)))
			}
			if !pr.HasBlockers && hasRepoTideErrBlockers {
				summaryParts = append(summaryParts, "ready · ⚠ tideErrLoopBlocker in repo")
			} else if !pr.HasBlockers {
				summaryParts = append(summaryParts, "ready")
			}

			// Default collapse state: ready PRs collapsed, blocker PRs expanded.
			// Only set default on first build (don't override user toggle).
			if _, hasState := m.prCollapsed[prKey]; !hasState {
				m.prCollapsed[prKey] = !pr.HasBlockers // collapse if ready, expand if blocked
			}

			arrow := IconExpanded
			if m.prCollapsed[prKey] {
				arrow = IconCollapsed
			}

			summary := strings.Join(summaryParts, " · ")
			prLine := fmt.Sprintf("    %s %s #%d [%s] %s  (%s)", arrow, icon, pr.Number, pr.Base, pr.Title, summary)
			m.rows = append(m.rows, queueRow{
				kind:     rowRepoSummary,
				repoName: repo,
				prKey:    prKey,
				text:     prLine,
				severity: prSev,
				url:      pr.PRURL,
			})

			if m.prCollapsed[prKey] {
				continue
			}

			// --- PR detail rows (collapsible) ---

			// Author + links.
			m.rows = append(m.rows, queueRow{
				kind: rowRepoDetail, repoName: repo, prKey: prKey,
				text: fmt.Sprintf("      Author: %s", pr.Author), severity: SeverityOK,
			})
			m.rows = append(m.rows, queueRow{
				kind: rowRepoDetail, repoName: repo, prKey: prKey,
				text: "      PR:     " + pr.PRURL, severity: SeverityOK, url: pr.PRURL,
			})
			m.rows = append(m.rows, queueRow{
				kind: rowRepoDetail, repoName: repo, prKey: prKey,
				text: "      Prow:   " + pr.ProwURL, severity: SeverityOK, url: pr.ProwURL,
			})

			// Label blockers.
			for _, label := range pr.LabelBlockers {
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      BLOCKED  label: %s", label), severity: SeverityIssue,
				})
			}

			// Review blocked.
			if pr.ReviewBlocked {
				reviewText := fmt.Sprintf("      REVIEWS  %d/%d GitHub approvals (need %d more)", pr.ApprovalCount, pr.RequiredReviews, pr.ReviewsShort)
				if isTideErrLoopBlocker {
					reviewText += " — enforce_admins blocks Tide merge (tideErrLoopBlocker, prow#134)"
				}
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: reviewText, severity: SeverityIssue,
				})
			}

			// Failing checks.
			for _, check := range pr.Checks.Failing {
				checkURL := check.URL
				if checkURL == "" {
					checkURL = pr.ProwURL
				}
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      FAILED   %s", check.Name), severity: SeverityIssue, url: checkURL,
				})
			}

			// Errored checks.
			for _, check := range pr.Checks.Errored {
				checkURL := check.URL
				if checkURL == "" {
					checkURL = pr.ProwURL
				}
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      ERROR    %s", check.Name), severity: SeverityIssue, url: checkURL,
				})
			}

			// In-progress checks.
			for _, check := range pr.Checks.InProgress {
				checkURL := check.URL
				if checkURL == "" {
					checkURL = pr.ProwURL
				}
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      RUNNING  %s", check.Name), severity: SeverityWarning, url: checkURL,
				})
			}

			// Pending checks.
			if len(pr.Checks.Pending) <= 5 {
				for _, check := range pr.Checks.Pending {
					m.rows = append(m.rows, queueRow{
						kind: rowRepoDetail, repoName: repo, prKey: prKey,
						text: fmt.Sprintf("      PENDING  %s", check.Name), severity: SeverityWarning, url: check.URL,
					})
				}
			} else {
				for _, check := range pr.Checks.Pending[:3] {
					m.rows = append(m.rows, queueRow{
						kind: rowRepoDetail, repoName: repo, prKey: prKey,
						text: fmt.Sprintf("      PENDING  %s", check.Name), severity: SeverityWarning, url: check.URL,
					})
				}
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      PENDING  ... and %d more", len(pr.Checks.Pending)-3), severity: SeverityWarning,
				})
			}

			// Tide status.
			switch {
			case pr.Checks.TideState == "SUCCESS" || pr.Checks.TideState == "EXPECTED":
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: "      TIDE     merge criteria met — merging soon", severity: SeverityOK, url: report.TideURL,
				})
			case pr.Checks.TideState == "PENDING" && len(pr.BranchBlockers) > 0:
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text:     fmt.Sprintf("      TIDE     waiting — review-blocked PRs ahead: %s", strings.Join(pr.BranchBlockers, ", ")),
					severity: SeverityWarning, url: report.TideURL,
				})
			case pr.Checks.TideState == "PENDING":
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: "      TIDE     waiting (another PR may be testing ahead in queue)", severity: SeverityInfo, url: report.TideURL,
				})
			default:
				m.rows = append(m.rows, queueRow{
					kind: rowRepoDetail, repoName: repo, prKey: prKey,
					text: fmt.Sprintf("      TIDE     %s", pr.Checks.TideState), severity: SeverityInfo, url: report.TideURL,
				})
			}

			// Ready status.
			if !pr.HasBlockers {
				// Check if any OTHER PR in this repo is a tideErrLoopBlocker (blocks entire Tide queue).
				var repoTideBlockers []string
				for _, blockerNum := range tideErrLoopBlockerPRs {
					if blockerNum != pr.Number {
						repoTideBlockers = append(repoTideBlockers, fmt.Sprintf("#%d", blockerNum))
					}
				}
				if len(repoTideBlockers) > 0 {
					m.rows = append(m.rows, queueRow{
						kind: rowRepoDetail, repoName: repo, prKey: prKey,
						text:     fmt.Sprintf("      ⚠ WARN   Tide may be blocked — tideErrLoopBlocker PRs in repo: %s (enforce_admins + prow#134)", strings.Join(repoTideBlockers, ", ")),
						severity: SeverityIssue,
					})
				}

				if len(pr.BranchBlockers) > 0 {
					m.rows = append(m.rows, queueRow{
						kind: rowRepoDetail, repoName: repo, prKey: prKey,
						text:     fmt.Sprintf("      READY    All checks passed — but review-blocked PRs may delay: %s", strings.Join(pr.BranchBlockers, ", ")),
						severity: SeverityWarning,
					})
				} else {
					m.rows = append(m.rows, queueRow{
						kind: rowRepoDetail, repoName: repo, prKey: prKey,
						text: "      READY    All checks passed — merge imminent", severity: SeverityOK,
					})
				}
			}
		}
	}

	// Show summary line for hidden healthy repos.
	if m.hideHealthy && healthyCount > 0 {
		label := "repo"
		if healthyCount > 1 {
			label = "repos"
		}
		m.rows = append(m.rows, queueRow{
			kind:     rowRepoDetail,
			text:     fmt.Sprintf("  %s %d healthy %s hidden (press a to show)", IconOK, healthyCount, label),
			severity: SeverityOK,
		})
	}

	// Show unchecked repos (in allRepos but not in queues and not loading).
	for _, repo := range m.allRepos {
		if _, hasResult := m.queues[repo]; hasResult {
			continue
		}
		if m.loading[repo] {
			continue
		}
		if m.ghAvailable {
			m.rows = append(m.rows, queueRow{
				kind:     rowGroupHeader,
				repoName: repo,
				text:     fmt.Sprintf("[m] %s  (not checked — press m or click [m])", repo),
				severity: SeverityOK,
			})
		} else {
			m.rows = append(m.rows, queueRow{
				kind:     rowRepoDetail,
				repoName: repo,
				text:     fmt.Sprintf("    %s  (requires gh CLI)", repo),
				severity: SeverityOK,
			})
		}
	}

	m.allQueueRows = make([]queueRow, len(m.rows))
	copy(m.allQueueRows, m.rows)
	m.applyFilter(m.filterQuery)
}

func (m *queueModel) applyFilter(query string) {
	m.filterQuery = query
	if query == "" {
		m.rows = m.allQueueRows
		m.clampCursor()
		return
	}
	var filtered []queueRow
	// Track which repos have matching children.
	repoHasMatch := make(map[string]bool)
	for _, row := range m.allQueueRows {
		if row.kind != rowGroupHeader && matchesFilter(row.text, query) {
			repoHasMatch[row.repoName] = true
		}
	}
	for _, row := range m.allQueueRows {
		switch row.kind {
		case rowGroupHeader:
			if repoHasMatch[row.repoName] || matchesFilter(row.text, query) {
				filtered = append(filtered, row)
			}
		case rowAction:
			filtered = append(filtered, row) // always show action row
		default:
			if matchesFilter(row.text, query) || repoHasMatch[row.repoName] {
				filtered = append(filtered, row)
			}
		}
	}
	m.rows = filtered
	m.cursor = 0
	m.scrollOff = 0
	m.clampCursor()
}

func (m queueModel) View(width, height int, filterQuery string) string {
	if len(m.rows) == 0 {
		msg := "  No merge queue data loaded. Press m on a repo or M to check all."
		return RowDimStyle.Render(msg)
	}

	var b strings.Builder

	// Sort mode and filter indicator.
	filterLabel := "showing all"
	if m.hideHealthy {
		filterLabel = "healthy hidden"
	}
	sortLabel := RowDimStyle.Render(fmt.Sprintf("  [sort: %s] [%s]", m.sortMode, filterLabel))
	b.WriteString(sortLabel)
	b.WriteString("\n")
	height--

	end := m.scrollOff + height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scrollOff; i < end; i++ {
		row := m.rows[i]
		line := padLineWithURL(row.text, row.url, width)

		// Check if this row belongs to a healthy repo (dim when showing all).
		isHealthyRepo := !m.hideHealthy && row.repoName != "" && m.repoIsHealthy(row.repoName)

		var styled string
		if i == m.cursor {
			styled = RowSelectedStyle.Render(line)
		} else if isHealthyRepo {
			styled = RowDimStyle.Render(highlightMatch(line, filterQuery))
		} else if row.kind == rowAction {
			styled = FooterKeyStyle.Render(line)
		} else if row.kind == rowGroupHeader && len(line) > 4 && line[:4] == "[m] " {
			// Style [m] prefix in accent, rest in severity color.
			styled = FooterKeyStyle.Render(line[:4]) + SeverityStyle(row.severity).Render(highlightMatch(line[4:], filterQuery))
		} else {
			styled = SeverityStyle(row.severity).Render(highlightMatch(line, filterQuery))
		}

		b.WriteString(styled)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return appendScrollIndicator(b.String(), m.cursor, len(m.rows), width)
}

func (m *queueModel) Update(msg tea.KeyMsg) tea.Cmd {
	if newCursor, handled := handleNavKeys(msg, m.cursor, len(m.rows)); handled {
		m.cursor = newCursor
		m.clampCursor()
		return nil
	}

	switch msg.String() {
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch {
			case row.kind == rowGroupHeader:
				m.collapsed[row.repoName] = !m.collapsed[row.repoName]
				m.buildRows()
			case row.kind == rowRepoSummary && row.prKey != "":
				m.prCollapsed[row.prKey] = !m.prCollapsed[row.prKey]
				m.buildRows()
			}
		}
	case "left", "h":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch {
			case row.kind == rowGroupHeader && row.repoName != "" && !m.collapsed[row.repoName]:
				m.collapsed[row.repoName] = true
				m.buildRows()
			case row.kind == rowRepoSummary && row.prKey != "" && !m.prCollapsed[row.prKey]:
				m.prCollapsed[row.prKey] = true
				m.buildRows()
			case row.kind == rowRepoDetail && row.prKey != "" && !m.prCollapsed[row.prKey]:
				m.prCollapsed[row.prKey] = true
				m.buildRows()
			}
		}
	case "right", "l":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			// Priority: expand collapsed items first, then open browser.
			switch {
			case row.kind == rowGroupHeader && row.repoName != "" && m.collapsed[row.repoName]:
				m.collapsed[row.repoName] = false
				m.buildRows()
			case row.kind == rowRepoSummary && row.prKey != "" && m.prCollapsed[row.prKey]:
				m.prCollapsed[row.prKey] = false
				m.buildRows()
			default:
				// Already expanded (or detail row) — open browser if URL exists.
				if row.url != "" {
					openBrowser(row.url)
				}
			}
		}
	case "o":
		// Open URL for current row.
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			if u := m.rows[m.cursor].url; u != "" {
				openBrowser(u)
			}
		}
	case "e":
		for repo := range m.collapsed {
			m.collapsed[repo] = false
		}
		for prKey := range m.prCollapsed {
			m.prCollapsed[prKey] = false
		}
		m.buildRows()
	case "c":
		for _, repo := range m.repoOrder {
			m.collapsed[repo] = true
		}
		for prKey := range m.prCollapsed {
			m.prCollapsed[prKey] = true
		}
		m.buildRows()
	case "s":
		m.sortMode = (m.sortMode + 1) % 4
		m.buildRows()
	case "a":
		m.hideHealthy = !m.hideHealthy
		m.buildRows()
	}

	m.clampCursor()
	return nil
}

func (m *queueModel) clampCursor() {
	m.cursor, m.scrollOff = clampScroll(m.cursor, m.scrollOff, len(m.rows))
}

// copyRepo returns plain text for a single repo's queue results.
func (m queueModel) copyRepo(repoName string) string {
	var b strings.Builder
	inRepo := false
	for _, row := range m.rows {
		if row.kind == rowGroupHeader && row.repoName == repoName {
			inRepo = true
		} else if row.kind == rowGroupHeader && row.repoName != repoName && inRepo {
			break
		}
		if row.kind == rowAction {
			continue
		}
		if inRepo {
			b.WriteString(row.text + "\n")
		}
	}
	return b.String()
}

// copyAll returns plain text for all queue results.
func (m queueModel) copyAll() string {
	var b strings.Builder
	for _, row := range m.rows {
		if row.kind == rowAction {
			continue
		}
		b.WriteString(row.text + "\n")
	}
	return b.String()
}
