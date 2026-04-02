package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type tideModel struct {
	groups      []RepoGroup
	collapsed   map[int]bool // group index -> collapsed
	allTideRows []tideRow    // unfiltered rows from buildRows
	rows        []tideRow
	filterQuery string
	cursor      int
	scrollOff   int
}

type tideRow struct {
	text     string
	severity Severity
	url      string
	isHeader bool
	groupIdx int
}

func newTideModel(groups []RepoGroup) tideModel {
	m := tideModel{
		groups:    groups,
		collapsed: make(map[int]bool),
	}
	m.buildRows()
	return m
}

func (m *tideModel) buildRows() {
	m.rows = nil

	for gi, g := range m.groups {
		// Group header with collapse indicator.
		arrow := IconExpanded
		if m.collapsed[gi] {
			arrow = IconCollapsed
		}
		headerText := fmt.Sprintf("%s %s (%d repos)", arrow, g.Name, len(g.Repos))
		m.rows = append(m.rows, tideRow{
			text:     headerText,
			severity: SeverityInfo,
			isHeader: true,
			groupIdx: gi,
		})

		if m.collapsed[gi] {
			continue
		}

		for _, repo := range g.Repos {
			parts := strings.SplitN(repo.Name, "/", 2)
			repoURL := ""
			if len(parts) == 2 {
				repoURL = fmt.Sprintf("https://github.com/openshift/release/blob/main/core-services/prow/02_config/%s/%s/_prowconfig.yaml", parts[0], parts[1])
			}

			if len(repo.Tide.IncludedBranches) > 0 {
				branches := strings.Join(repo.Tide.IncludedBranches, ", ")
				m.rows = append(m.rows, tideRow{
					text:     fmt.Sprintf("  %-40s %s", repo.Name, branches),
					severity: SeverityOK,
					url:      repoURL,
					groupIdx: gi,
				})
			} else {
				m.rows = append(m.rows, tideRow{
					text:     fmt.Sprintf("  %-40s ⚠ No includedBranches", repo.Name),
					severity: SeverityWarning,
					url:      repoURL,
					groupIdx: gi,
				})
			}
		}
	}

	m.allTideRows = make([]tideRow, len(m.rows))
	copy(m.allTideRows, m.rows)
	m.applyFilter(m.filterQuery)
}

func (m *tideModel) applyFilter(query string) {
	m.filterQuery = query
	if query == "" {
		m.rows = m.allTideRows
		m.clampCursor()
		return
	}
	var filtered []tideRow
	groupHasMatch := make(map[int]bool)
	for _, row := range m.allTideRows {
		if !row.isHeader && matchesFilter(row.text, query) {
			groupHasMatch[row.groupIdx] = true
		}
	}
	for _, row := range m.allTideRows {
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

func (m tideModel) View(width, height int, filterQuery string) string {
	if len(m.rows) == 0 {
		return RowDimStyle.Render("  No tide data available.")
	}

	var b strings.Builder

	end := m.scrollOff + height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scrollOff; i < end; i++ {
		row := m.rows[i]
		line := padLineWithURL(row.text, row.url, width)

		var styled string
		if i == m.cursor {
			styled = RowSelectedStyle.Render(line)
		} else if row.isHeader {
			styled = SectionHeaderStyle.Render(highlightMatch(line, filterQuery))
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

func (m *tideModel) Update(msg tea.KeyMsg) tea.Cmd {
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
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			if row.url != "" {
				openBrowser(row.url)
				return nil
			}
			if row.isHeader && m.collapsed[row.groupIdx] {
				m.collapsed[row.groupIdx] = false
				m.buildRows()
			}
		}
	case "o":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			if u := m.rows[m.cursor].url; u != "" {
				openBrowser(u)
			}
		}
	case "e":
		for gi := range m.collapsed {
			m.collapsed[gi] = false
		}
		m.buildRows()
	case "c":
		for gi := range m.groups {
			m.collapsed[gi] = true
		}
		m.buildRows()
	}

	m.clampCursor()
	return nil
}

func (m *tideModel) clampCursor() {
	m.cursor, m.scrollOff = clampScroll(m.cursor, m.scrollOff, len(m.rows))
}

// copyGroup returns plain text for a single group's tide data.
func (m tideModel) copyGroup(groupIdx int) string {
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

// copyAll returns plain text for all tide data.
func (m tideModel) copyAll() string {
	var b strings.Builder
	for _, row := range m.rows {
		b.WriteString(row.text + "\n")
	}
	return b.String()
}
