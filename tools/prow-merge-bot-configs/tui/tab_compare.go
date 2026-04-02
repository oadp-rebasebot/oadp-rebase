package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type compareModel struct {
	report           *AuditReport
	allCompareRows   []compareRow // unfiltered rows from buildRows
	rows             []compareRow
	filterQuery      string
	cursor           int
	scrollOff        int
	showOutliersOnly bool
	collapsed        map[int]bool // group index -> collapsed
}

type compareRow struct {
	text      string
	severity  Severity
	isOutlier bool // true for outlier rows — uses distinct color
	isHeader  bool // true for group headers (collapsible)
	groupIdx  int  // which group this row belongs to
}

func newCompareModel(report *AuditReport) compareModel {
	m := compareModel{
		report:    report,
		collapsed: make(map[int]bool),
	}
	m.buildRows()
	return m
}

func (m *compareModel) buildRows() {
	m.rows = nil

	if m.report == nil || m.report.Comparison == nil {
		m.rows = append(m.rows, compareRow{
			text:     "  No comparison data available.",
			severity: SeverityInfo,
		})
		return
	}

	for gi, groupDef := range DefaultGroups() {
		groupComparison, ok := m.report.Comparison[groupDef.Type]
		if !ok {
			continue
		}

		// Section header with collapse indicator.
		arrow := IconExpanded
		if m.collapsed[gi] {
			arrow = IconCollapsed
		}
		headerText := fmt.Sprintf("%s %s", arrow, groupDef.Name)
		m.rows = append(m.rows, compareRow{
			text:     headerText,
			severity: SeverityInfo,
			isHeader: true,
			groupIdx: gi,
		})

		if m.collapsed[gi] {
			continue
		}

		hasOutliers := false

		for field, fc := range groupComparison {
			if len(fc.Outliers) == 0 {
				if m.showOutliersOnly {
					continue
				}
				m.rows = append(m.rows, compareRow{
					text:     fmt.Sprintf("  %s (all: %s)", field, fc.Majority),
					severity: SeverityOK,
					groupIdx: gi,
				})
				continue
			}

			hasOutliers = true

			m.rows = append(m.rows, compareRow{
				text:     fmt.Sprintf("  %s (majority: %s)", field, fc.Majority),
				severity: SeverityWarning,
				groupIdx: gi,
			})

			for _, outlier := range fc.Outliers {
				m.rows = append(m.rows, compareRow{
					text:      fmt.Sprintf("    %s %s: %s (differs from majority: %s)", IconOutlier, outlier.Repo, outlier.Value, fc.Majority),
					severity:  SeverityWarning,
					isOutlier: true,
					groupIdx:  gi,
				})
			}
		}

		if !hasOutliers && !m.showOutliersOnly {
			m.rows = append(m.rows, compareRow{
				text: "  All repos consistent", severity: SeverityOK, groupIdx: gi,
			})
		} else if !hasOutliers && m.showOutliersOnly {
			m.rows = append(m.rows, compareRow{
				text: "  No outliers in this group", severity: SeverityOK, groupIdx: gi,
			})
		}

		m.rows = append(m.rows, compareRow{text: "", severity: SeverityOK})
	}

	m.allCompareRows = make([]compareRow, len(m.rows))
	copy(m.allCompareRows, m.rows)
	m.applyFilter(m.filterQuery)
}

func (m *compareModel) applyFilter(query string) {
	m.filterQuery = query
	if query == "" {
		m.rows = m.allCompareRows
		m.clampCursor()
		return
	}
	var filtered []compareRow
	groupHasMatch := make(map[int]bool)
	for _, row := range m.allCompareRows {
		if !row.isHeader && matchesFilter(row.text, query) {
			groupHasMatch[row.groupIdx] = true
		}
	}
	for _, row := range m.allCompareRows {
		if row.isHeader {
			if groupHasMatch[row.groupIdx] {
				filtered = append(filtered, row)
			}
		} else if matchesFilter(row.text, query) {
			filtered = append(filtered, row)
		}
	}
	m.rows = filtered
	m.cursor = 0
	m.scrollOff = 0
	m.clampCursor()
}

func (m compareModel) View(width, height int, filterQuery string) string {
	if len(m.rows) == 0 {
		return RowDimStyle.Render("  No comparison data available.")
	}

	var b strings.Builder

	// Show mode indicator.
	mode := "Full view"
	if m.showOutliersOnly {
		mode = "Outliers only"
	}
	modeStr := RowDimStyle.Render(fmt.Sprintf("  [%s]", mode))
	b.WriteString(modeStr)
	b.WriteString("\n")
	height-- // account for mode line

	end := m.scrollOff + height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scrollOff; i < end; i++ {
		row := m.rows[i]
		line := padLine(row.text, width)

		var styled string
		if i == m.cursor {
			styled = RowSelectedStyle.Render(line)
		} else if row.isHeader {
			styled = SectionHeaderStyle.Render(highlightMatch(line, filterQuery))
		} else if row.isOutlier {
			styled = RowOutlierStyle.Render(highlightMatch(line, filterQuery))
		} else {
			styled = SeverityStyle(row.severity).Render(highlightMatch(line, filterQuery))
		}

		b.WriteString(styled)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *compareModel) Update(msg tea.KeyMsg) tea.Cmd {
	if newCursor, handled := handleNavKeys(msg, m.cursor, len(m.rows)); handled {
		m.cursor = newCursor
		m.clampCursor()
		return nil
	}

	switch msg.String() {
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isHeader {
			gi := m.rows[m.cursor].groupIdx
			m.collapsed[gi] = !m.collapsed[gi]
			m.buildRows()
		}
	case "left", "h":
		if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isHeader {
			gi := m.rows[m.cursor].groupIdx
			if !m.collapsed[gi] {
				m.collapsed[gi] = true
				m.buildRows()
			}
		}
	case "right", "l":
		if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isHeader {
			gi := m.rows[m.cursor].groupIdx
			if m.collapsed[gi] {
				m.collapsed[gi] = false
				m.buildRows()
			}
		}
	case "e":
		for gi := range m.collapsed {
			m.collapsed[gi] = false
		}
		m.buildRows()
	case "c":
		for gi := range DefaultGroups() {
			m.collapsed[gi] = true
		}
		m.buildRows()
	case "o":
		m.showOutliersOnly = !m.showOutliersOnly
		m.buildRows()
	}

	m.clampCursor()
	return nil
}

func (m *compareModel) clampCursor() {
	m.cursor, m.scrollOff = clampScroll(m.cursor, m.scrollOff, len(m.rows))
}

// copyGroup returns plain text for a single group's comparison data.
func (m compareModel) copyGroup(groupIdx int) string {
	var b strings.Builder
	inGroup := false
	for _, row := range m.rows {
		if row.isHeader && row.groupIdx == groupIdx {
			inGroup = true
		} else if row.isHeader && row.groupIdx != groupIdx && inGroup {
			break
		}
		if inGroup {
			b.WriteString(row.text + "\n")
		}
	}
	return b.String()
}

// copyAll returns plain text for all comparison data.
func (m compareModel) copyAll() string {
	var b strings.Builder
	for _, row := range m.rows {
		b.WriteString(row.text + "\n")
	}
	return b.String()
}
