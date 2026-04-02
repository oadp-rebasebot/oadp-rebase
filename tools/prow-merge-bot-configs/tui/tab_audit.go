package main

import (
	"fmt"
	"maps"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// lipgloss is used indirectly via padLineWithURL and theme styles.

// Row types for the flat row list.
type rowKind int

const (
	rowGroupHeader rowKind = iota
	rowGroupDesc
	rowRepoSummary
	rowRepoDetail
	rowAction       // clickable action button
	rowBranchHeader // ci-operator branch header (CI tab)
)

type auditRow struct {
	kind         rowKind
	groupIdx     int
	repoName     string
	text         string   // pre-rendered content (without cursor highlight)
	severity     Severity // for coloring
	btnCol       int      // column where [m] button starts (-1 if none)
	url          string   // if set, double-click/right-arrow opens this URL
	diffBadge    string   // "NEW", "FIXED", or "" for diff-on-refresh
	suggestedFix string   // for 'c' clipboard copy
}

type auditModel struct {
	groups       []RepoGroup
	configs      map[string]*RepoConfig // for field line number lookups
	prevFindings map[string][]Finding    // previous audit findings for diff badges
	allRows      []auditRow             // unfiltered rows from buildRows
	rows         []auditRow
	filterQuery  string
	cursor       int
	scrollOff    int
	collapsed    map[int]bool    // group index -> collapsed
	expanded     map[string]bool // repo name -> detail expanded
}

// auditViewState captures the visual state of the audit tab for restoration after refresh.
type auditViewState struct {
	collapsedGroups map[string]bool // group Name -> collapsed
	expanded        map[string]bool // repo name -> detail expanded
	cursorRowKey    string          // logical key for cursor row
	scrollOff       int
	filterQuery     string
}

// rowKey returns a stable identifier for a row based on its kind and logical position.
func (m *auditModel) rowKey(r auditRow) string {
	groupName := ""
	if r.groupIdx >= 0 && r.groupIdx < len(m.groups) {
		groupName = m.groups[r.groupIdx].Name
	}
	return fmt.Sprintf("%d:%s:%s", r.kind, groupName, r.repoName)
}

func (m *auditModel) saveViewState() auditViewState {
	s := auditViewState{
		collapsedGroups: make(map[string]bool),
		expanded:        make(map[string]bool),
		scrollOff:       m.scrollOff,
		filterQuery:     m.filterQuery,
	}
	// Convert index-based collapsed map to name-based.
	for idx, v := range m.collapsed {
		if v && idx >= 0 && idx < len(m.groups) {
			s.collapsedGroups[m.groups[idx].Name] = true
		}
	}
	// Copy expanded map (already keyed by repo name).
	maps.Copy(s.expanded, m.expanded)
	// Save cursor row key.
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		s.cursorRowKey = m.rowKey(m.rows[m.cursor])
	}
	return s
}

func (m *auditModel) restoreViewState(s auditViewState) {
	// Restore collapsed state: convert group names back to indices.
	m.collapsed = make(map[int]bool)
	for gi, g := range m.groups {
		if s.collapsedGroups[g.Name] {
			m.collapsed[gi] = true
		}
	}
	// Restore expanded state.
	m.expanded = s.expanded

	// Rebuild rows with restored collapse/expand state.
	m.buildRows()

	// Restore filter.
	if s.filterQuery != "" {
		m.applyFilter(s.filterQuery)
	}

	// Restore cursor position by finding the matching row key.
	// If not found, keep cursor at same index, clamped to valid range.
	if s.cursorRowKey != "" {
		for i, r := range m.rows {
			if m.rowKey(r) == s.cursorRowKey {
				m.cursor = i
				break
			}
		}
	}

	m.scrollOff = s.scrollOff
	m.clampCursor()
}

func newAuditModel(groups []RepoGroup, configs map[string]*RepoConfig, prevFindings map[string][]Finding) auditModel {
	m := auditModel{
		groups:       groups,
		configs:      configs,
		prevFindings: prevFindings,
		collapsed:    make(map[int]bool),
		expanded:     make(map[string]bool),
	}
	m.buildRows()
	return m
}

func (m *auditModel) buildRows() {
	m.buildAllRows()
	m.applyFilter(m.filterQuery)
}

func (m *auditModel) buildAllRows() {
	m.allRows = nil

	for gi, g := range m.groups {
		// Group header row.
		arrow := "▼"
		if m.collapsed[gi] {
			arrow = "▶"
		}
		headerText := fmt.Sprintf("%s %s (%d repos)", arrow, g.Name, len(g.Repos))
		m.allRows = append(m.allRows, auditRow{
			kind:     rowGroupHeader,
			groupIdx: gi,
			text:     headerText,
			severity: SeverityOK,
		})

		if m.collapsed[gi] {
			continue
		}

		// Group description — split on newlines so each line is its own row.
		if g.Description != "" {
			for _, descLine := range strings.Split(g.Description, "\n") {
				m.allRows = append(m.allRows, auditRow{
					kind:     rowGroupDesc,
					groupIdx: gi,
					text:     "  " + descLine,
					severity: SeverityOK,
				})
			}
		}

		// Repos in group.
		for _, repo := range g.Repos {
			worst := repo.WorstSeverity()
			icon := SeverityIcon(worst)
			statusLabel := severityLabel(worst)
			keyDetail := buildKeyDetail(g.Type, repo)

			// Flag missing plugins in the summary line.
			var missingPlugins []string
			if !repo.Plugins.HasApproveSection && !repo.Plugins.HasApprovePlugin {
				missingPlugins = append(missingPlugins, "approve")
			}
			if !repo.Plugins.HasLgtmSection {
				missingPlugins = append(missingPlugins, "lgtm")
			}
			pluginHint := ""
			if len(missingPlugins) > 0 {
				pluginHint = fmt.Sprintf(" [missing: %s]", strings.Join(missingPlugins, ","))
			}

			// Compute diff badge for repo summary.
			var repoDiffBadge string
			if m.prevFindings != nil {
				newCount, fixedCount := repoDiffCounts(repo.Findings, m.prevFindings[repo.Name])
				repoDiffBadge = formatDiffBadge(newCount, fixedCount)
			}

			summaryText := fmt.Sprintf("  [m] %-40s %s %-8s %s%s", repo.Name, icon, statusLabel, keyDetail, pluginHint)
			if repoDiffBadge != "" {
				summaryText += " " + repoDiffBadge
			}
			// URL to the repo's prow config directory in openshift/release.
			parts := strings.SplitN(repo.Name, "/", 2)
			repoURL := ""
			if len(parts) == 2 {
				repoURL = fmt.Sprintf("https://github.com/openshift/release/tree/main/core-services/prow/02_config/%s/%s", parts[0], parts[1])
			}
			m.allRows = append(m.allRows, auditRow{
				kind:      rowRepoSummary,
				groupIdx:  gi,
				repoName:  repo.Name,
				text:      summaryText,
				severity:  worst,
				btnCol:    2, // [m] starts at column 2
				url:       repoURL,
				diffBadge: repoDiffBadge,
			})

			// Expanded detail rows.
			if m.expanded[repo.Name] {
				rc := m.configs[repo.Name]
				details := buildDetailRows(repo, rc)
				// Build set of current finding keys for diff.
				currentKeys := make(map[string]bool)
				for _, f := range repo.Findings {
					currentKeys[f.Key()] = true
				}
				// Build set of previous finding keys.
				prevKeys := make(map[string]bool)
				if m.prevFindings != nil {
					for _, f := range m.prevFindings[repo.Name] {
						prevKeys[f.Key()] = true
					}
				}
				// Map field name to finding key for badge lookup.
				fieldToKey := make(map[string]string)
				for _, f := range repo.Findings {
					fieldToKey[f.Field] = f.Key()
				}
				for _, d := range details {
					badge := ""
					if m.prevFindings != nil {
						if key, ok := fieldToKey[d.findingField]; ok && !prevKeys[key] {
							badge = "NEW"
						}
					}
					m.allRows = append(m.allRows, auditRow{
						kind:         rowRepoDetail,
						groupIdx:     gi,
						repoName:     repo.Name,
						text:         d.text,
						severity:     d.severity,
						url:          d.url,
						diffBadge:    badge,
						suggestedFix: d.suggestedFix,
					})
					if d.suggestedFix != "" {
						sugLines := strings.Split(d.suggestedFix, "\n")
						for _, sl := range sugLines {
							m.allRows = append(m.allRows, auditRow{
								kind:         rowRepoDetail,
								groupIdx:     gi,
								repoName:     repo.Name,
								text:         "    |     " + sl,
								severity:     SeverityInfo,
								suggestedFix: d.suggestedFix,
							})
						}
					}
				}
				// Append synthetic FIXED rows for findings that were resolved.
				if m.prevFindings != nil {
					for _, pf := range m.prevFindings[repo.Name] {
						if !currentKeys[pf.Key()] && pf.Severity > SeverityOK {
							fixedText := fmt.Sprintf("    ├─ %-28s FIXED", pf.Field+":")
							m.allRows = append(m.allRows, auditRow{
								kind:      rowRepoDetail,
								groupIdx:  gi,
								repoName:  repo.Name,
								text:      fixedText,
								severity:  SeverityOK,
								diffBadge: "FIXED",
							})
						}
					}
				}
			}
		}
	}
}

func (m *auditModel) applyFilter(query string) {
	m.filterQuery = query
	if query == "" {
		m.rows = m.allRows
		m.clampCursor()
		return
	}
	var filtered []auditRow
	groupHasMatch := make(map[int]bool)
	for _, row := range m.allRows {
		if row.kind != rowGroupHeader && row.kind != rowGroupDesc && matchesFilter(row.text, query) {
			groupHasMatch[row.groupIdx] = true
		}
	}
	for _, row := range m.allRows {
		switch row.kind {
		case rowGroupHeader, rowGroupDesc:
			if groupHasMatch[row.groupIdx] {
				filtered = append(filtered, row)
			}
		default:
			if matchesFilter(row.text, query) {
				filtered = append(filtered, row)
			}
		}
	}
	m.rows = filtered
	m.cursor = 0
	m.scrollOff = 0
	m.clampCursor()
}

func buildKeyDetail(groupType string, repo RepoAudit) string {
	switch groupType {
	case "upstream-rebase":
		fp := repo.Fields["allow_force_pushes"]
		if fp == "" {
			fp = "NOT_SET"
		}
		return "force_push=" + fp
	default:
		ea := repo.Fields["enforce_admins"]
		if ea == "" {
			ea = "NOT_SET"
		}
		rc := repo.Fields["required_approving_review_count"]
		if rc == "" {
			rc = "NOT_SET"
		}
		return fmt.Sprintf("enforce_admins=%s review_count=%s", ea, rc)
	}
}

type detailLine struct {
	text         string
	severity     Severity
	url          string
	findingField string // field name for diff badge lookup
	suggestedFix string // YAML suggestion for clipboard
}

func buildDetailRows(repo RepoAudit, rc *RepoConfig) []detailLine {
	var lines []detailLine

	// Field order for display.
	fieldOrder := []string{
		"require_self_approval",
		"review_acts_as_lgtm",
		"enforce_admins",
		"required_approving_review_count",
		"dismiss_stale_reviews",
		"allow_force_pushes",
		"merge_method",
	}

	// Build a finding map for quick lookup.
	findingMap := make(map[string]Finding)
	for _, f := range repo.Findings {
		findingMap[f.Field] = f
	}

	totalFields := len(fieldOrder) + 3 // +3 for plugins, tide, and owners
	idx := 0

	for _, field := range fieldOrder {
		val := repo.Fields[field]
		if val == "" {
			val = "NOT_SET"
		}

		checkIcon := " "
		sev := SeverityOK
		if f, ok := findingMap[field]; ok {
			sev = f.Severity
			checkIcon = SeverityIcon(sev)
		}

		prefix := "    ├─"
		idx++
		if idx == totalFields {
			prefix = "    └─"
		}

		// Link to the line in the config file where this field is defined.
		fieldURL := ""
		if rc != nil && val != "NOT_SET" && val != "MISSING_FILE" {
			fieldURL = FieldURL(rc, field)
		}

		// Attach suggested fix from finding if present.
		var fieldSuggestion string
		if f, ok := findingMap[field]; ok && f.SuggestedFix != "" {
			fieldSuggestion = f.SuggestedFix
		}

		line := fmt.Sprintf("%s %-28s %-10s %s", prefix, field+":", val, checkIcon)
		lines = append(lines, detailLine{text: line, severity: sev, url: fieldURL, findingField: field, suggestedFix: fieldSuggestion})
	}

	// Plugins row.
	idx++
	pluginPrefix := "    ├─"
	if idx == totalFields {
		pluginPrefix = "    └─"
	}
	approveStr := "✗"
	approveSev := SeverityIssue
	if repo.Plugins.HasApprovePlugin || repo.Plugins.HasApproveSection {
		approveStr = "✓"
		approveSev = SeverityOK
	}
	lgtmStr := "✗"
	lgtmSev := SeverityIssue
	if repo.Plugins.HasLgtmSection {
		lgtmStr = "✓"
		lgtmSev = SeverityOK
	}
	pluginSev := SeverityOK
	if approveSev == SeverityIssue || lgtmSev == SeverityIssue {
		pluginSev = SeverityIssue
	}
	// Combine plugin suggestions.
	var pluginSuggestions []string
	for _, pf := range []string{"approve_section", "lgtm_section", "approve_plugin"} {
		if f, ok := findingMap[pf]; ok && f.SuggestedFix != "" {
			pluginSuggestions = append(pluginSuggestions, f.SuggestedFix)
		}
	}
	pluginSuggestedFix := strings.Join(pluginSuggestions, "\n\n")

	pluginLine := fmt.Sprintf("%s Plugins: approve %s  lgtm %s", pluginPrefix, approveStr, lgtmStr)
	lines = append(lines, detailLine{text: pluginLine, severity: pluginSev, suggestedFix: pluginSuggestedFix})

	// Tide row.
	idx++
	tidePrefix := "    ├─"
	if idx == totalFields {
		tidePrefix = "    └─"
	}
	tideBranches := "none"
	if len(repo.Tide.IncludedBranches) > 0 {
		tideBranches = strings.Join(repo.Tide.IncludedBranches, ", ")
	}
	// Combine tide suggestions.
	var tideSuggestions []string
	for _, tf := range []string{"tide_labels", "tide_missing_labels"} {
		if f, ok := findingMap[tf]; ok && f.SuggestedFix != "" {
			tideSuggestions = append(tideSuggestions, f.SuggestedFix)
		}
	}
	tideSuggestedFix := strings.Join(tideSuggestions, "\n\n")

	tideLine := fmt.Sprintf("%s Tide: %s", tidePrefix, tideBranches)
	lines = append(lines, detailLine{text: tideLine, severity: SeverityOK, suggestedFix: tideSuggestedFix})

	// OWNERS row.
	idx++
	ownersPrefix := "    ├─"
	if idx == totalFields {
		ownersPrefix = "    └─"
	}
	var ownersLine string
	var ownersURL string
	ownersSev := SeverityOK
	if repo.Owners.HasFile {
		ownersLine = fmt.Sprintf("%s OWNERS: %s (%d approvers, %d reviewers)",
			ownersPrefix, repo.Owners.FilePath, len(repo.Owners.Approvers), len(repo.Owners.Reviewers))
		branch := "main"
		if rc != nil && rc.DefaultBranch != "" {
			branch = rc.DefaultBranch
		}
		ownersURL = fmt.Sprintf("https://github.com/%s/blob/%s/%s", repo.Name, branch, repo.Owners.FilePath)
	} else {
		ownersLine = fmt.Sprintf("%s OWNERS: missing", ownersPrefix)
		ownersSev = SeverityWarning
	}
	lines = append(lines, detailLine{text: ownersLine, severity: ownersSev, url: ownersURL})

	return lines
}

func severityLabel(s Severity) string {
	switch s {
	case SeverityOK:
		return "OK"
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARN"
	case SeverityIssue:
		return "ISSUE"
	default:
		return ""
	}
}

func (m auditModel) View(width, height int, filterQuery string) string {
	if len(m.rows) == 0 {
		return RowDimStyle.Render("  No audit data available.")
	}

	var b strings.Builder

	end := m.scrollOff + height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	const btnPrefixLen = 4 // len("[m] ")

	for i := m.scrollOff; i < end; i++ {
		row := m.rows[i]
		line := padLineWithURL(row.text, row.url, width)

		// Append diff badge to line if present.
		if row.diffBadge != "" {
			line = strings.TrimRight(line, " ")
			badge := " [" + row.diffBadge + "]"
			line += badge
			// Re-pad to width.
			if len(line) < width {
				line += strings.Repeat(" ", width-len(line))
			}
		}

		var styled string
		if i == m.cursor {
			styled = RowSelectedStyle.Render(line)
		} else if row.kind == rowRepoSummary && row.btnCol >= 0 {
			// Render [m] prefix in accent, rest in severity color.
			// line starts with "  [m] ..." — split at btnCol + btnPrefixLen
			// Use rune-based slicing for UTF-8 safety.
			btnEnd := row.btnCol + btnPrefixLen
			runes := []rune(line)
			if btnEnd > len(runes) {
				btnEnd = len(runes)
			}
			btnPart := string(runes[:btnEnd])
			restPart := highlightMatch(string(runes[btnEnd:]), filterQuery)
			styled = FooterKeyStyle.Render(btnPart) + SeverityStyle(row.severity).Render(restPart)
		} else if row.diffBadge == "FIXED" {
			styled = SeverityStyle(SeverityOK).Render(highlightMatch(line, filterQuery))
		} else {
			hl := highlightMatch(line, filterQuery)
			switch row.kind {
			case rowGroupHeader:
				styled = SectionHeaderStyle.Render(hl)
			case rowGroupDesc:
				styled = RowDimStyle.Render(hl)
			case rowRepoSummary, rowRepoDetail:
				styled = SeverityStyle(row.severity).Render(hl)
			default:
				styled = hl
			}
		}

		b.WriteString(styled)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return appendScrollIndicator(b.String(), m.cursor, len(m.rows), width)
}

func (m *auditModel) Update(msg tea.KeyMsg) tea.Cmd {
	if newCursor, handled := handleNavKeys(msg, m.cursor, len(m.rows)); handled {
		m.cursor = newCursor
		m.clampCursor()
		return nil
	}

	switch msg.String() {
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch row.kind {
			case rowGroupHeader:
				m.collapsed[row.groupIdx] = !m.collapsed[row.groupIdx]
				m.buildRows()
			case rowRepoSummary:
				m.expanded[row.repoName] = !m.expanded[row.repoName]
				m.buildRows()
			}
		}
	case "left", "h":
		// Collapse: if on group header, collapse group; if on repo, collapse detail.
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch row.kind {
			case rowGroupHeader:
				if !m.collapsed[row.groupIdx] {
					m.collapsed[row.groupIdx] = true
					m.buildRows()
				}
			case rowRepoSummary:
				if m.expanded[row.repoName] {
					m.expanded[row.repoName] = false
					m.buildRows()
				}
			case rowRepoDetail:
				// Collapse the parent repo.
				if row.repoName != "" && m.expanded[row.repoName] {
					m.expanded[row.repoName] = false
					m.buildRows()
				}
			}
		}
	case "right", "l":
		// Expand: if on group header, expand group; if on repo, expand detail.
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch row.kind {
			case rowGroupHeader:
				if m.collapsed[row.groupIdx] {
					m.collapsed[row.groupIdx] = false
					m.buildRows()
				}
			case rowRepoSummary:
				if !m.expanded[row.repoName] {
					m.expanded[row.repoName] = true
					m.buildRows()
				}
			}
		}
	case "o":
		// Open config URL for current row.
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			if u := m.rows[m.cursor].url; u != "" {
				openBrowser(u)
			}
		}
	case "e":
		// Expand all groups.
		for gi := range m.groups {
			m.collapsed[gi] = false
		}
		m.buildRows()
	case "C":
		// Collapse all groups (Shift+C).
		for gi := range m.groups {
			m.collapsed[gi] = true
		}
		m.buildRows()
	}

	m.clampCursor()
	return nil
}

func (m *auditModel) clampCursor() {
	m.cursor, m.scrollOff = clampScroll(m.cursor, m.scrollOff, len(m.rows))
}

// CursorRepo returns the repo name at the cursor, or "".
func (m auditModel) CursorRepo() string {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].repoName
	}
	return ""
}

// ClickedMButton checks if a click at the given body-relative row and column
// hit the [m] button prefix. Returns the repo name if so, or "".
func (m auditModel) ClickedMButton(bodyRow, col int) string {
	idx := m.scrollOff + bodyRow
	if idx < 0 || idx >= len(m.rows) {
		return ""
	}
	row := m.rows[idx]
	if row.kind != rowRepoSummary || row.btnCol < 0 {
		return ""
	}
	// [m] is at columns btnCol..btnCol+3 (i.e. "[m] ")
	if col >= row.btnCol && col < row.btnCol+4 {
		return row.repoName
	}
	return ""
}

// CursorSuggestion returns the suggested fix YAML at the cursor, or "".
func (m auditModel) CursorSuggestion() string {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].suggestedFix
	}
	return ""
}

// CursorOnGroup returns true if the cursor is on a group header.
func (m auditModel) CursorOnGroup() bool {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].kind == rowGroupHeader
	}
	return false
}

// snapshotFindings extracts a map of repo name → findings from the current report.
func snapshotFindings(report *AuditReport) map[string][]Finding {
	result := make(map[string][]Finding)
	for _, g := range report.Groups {
		for _, r := range g.Repos {
			findings := make([]Finding, len(r.Findings))
			copy(findings, r.Findings)
			result[r.Name] = findings
		}
	}
	return result
}

// repoDiffCounts compares current and previous findings for a repo,
// returning the count of new and fixed findings.
func repoDiffCounts(current, previous []Finding) (newCount, fixedCount int) {
	prevKeys := make(map[string]bool, len(previous))
	for _, f := range previous {
		prevKeys[f.Key()] = true
	}
	curKeys := make(map[string]bool, len(current))
	for _, f := range current {
		curKeys[f.Key()] = true
		if !prevKeys[f.Key()] {
			newCount++
		}
	}
	for _, f := range previous {
		if !curKeys[f.Key()] && f.Severity > SeverityOK {
			fixedCount++
		}
	}
	return
}

// formatDiffBadge formats a diff badge string like "[+2 new, -1 fixed]".
func formatDiffBadge(newCount, fixedCount int) string {
	if newCount == 0 && fixedCount == 0 {
		return ""
	}
	var parts []string
	if newCount > 0 {
		parts = append(parts, fmt.Sprintf("+%d new", newCount))
	}
	if fixedCount > 0 {
		parts = append(parts, fmt.Sprintf("-%d fixed", fixedCount))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// diffSummaryFlash computes an overall diff summary for the flash message.
func diffSummaryFlash(report *AuditReport, prevFindings map[string][]Finding) string {
	var totalNew, totalFixed int
	for _, g := range report.Groups {
		for _, r := range g.Repos {
			n, f := repoDiffCounts(r.Findings, prevFindings[r.Name])
			totalNew += n
			totalFixed += f
		}
	}
	if totalNew == 0 && totalFixed == 0 {
		return "Refresh: no changes"
	}
	return "Refresh: " + formatDiffBadge(totalNew, totalFixed)
}
