package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ciDoneMsg signals that CI data fetching is complete.
type ciDoneMsg struct {
	ciInfos map[string]*CIInfo
}

type ciRow struct {
	kind      rowKind
	groupIdx  int
	repoName  string
	branchKey string // "repo:branch" for branch rows and their children
	text      string
	severity  Severity
	url       string
}

type ciModel struct {
	groups           []RepoGroup
	ciInfos          map[string]*CIInfo
	allCIRows        []ciRow // unfiltered rows from buildRows
	rows             []ciRow
	filterQuery      string
	cursor           int
	scrollOff        int
	collapsed        map[int]bool    // group index -> collapsed
	expanded         map[string]bool // repo name -> jobs expanded
	expandedBranches map[string]bool // "repo:branch" -> branch tests expanded
	loading          bool
}

func newCIModel(groups []RepoGroup, ciInfos map[string]*CIInfo) ciModel {
	m := ciModel{
		groups:           groups,
		ciInfos:          ciInfos,
		collapsed:        make(map[int]bool),
		expanded:         make(map[string]bool),
		expandedBranches: make(map[string]bool),
	}
	m.buildRows()
	return m
}

func (m *ciModel) buildRows() {
	m.rows = nil

	for gi, g := range m.groups {
		// Group header.
		arrow := IconExpanded
		if m.collapsed[gi] {
			arrow = IconCollapsed
		}
		headerText := fmt.Sprintf("%s %s (%d repos)", arrow, g.Name, len(g.Repos))
		m.rows = append(m.rows, ciRow{
			kind:     rowGroupHeader,
			groupIdx: gi,
			text:     headerText,
			severity: SeverityOK,
		})

		if m.collapsed[gi] {
			continue
		}

		for _, repo := range g.Repos {
			ci := m.ciInfos[repo.Name]

			// Check for error/timeout first.
			if ci != nil && ci.Error != "" {
				summaryText := fmt.Sprintf("  %s %-40s %s", IconWarn, repo.Name, ci.Error)
				repoURL := fmt.Sprintf("https://github.com/%s", repo.Name)
				m.rows = append(m.rows, ciRow{
					kind:     rowRepoSummary,
					groupIdx: gi,
					repoName: repo.Name,
					text:     summaryText,
					severity: SeverityWarning,
					url:      repoURL,
				})
				continue
			}

			// Build repo summary line.
			goVer := "?"
			prowIcon := IconIssue
			actionsIcon := IconIssue
			prowSev := SeverityIssue
			actionsSev := SeverityIssue

			prowLabel := "Prow"
			if ci != nil {
				if ci.GoVersion != "" {
					goVer = ci.GoVersion
				}
				if ci.HasProw {
					prowIcon = IconOK
					prowSev = SeverityOK
					if ci.HasCIOperator && len(ci.ProwJobs) == 0 {
						prowLabel = fmt.Sprintf("Prow (ci-operator, %d configs)", ci.CIOperatorConfigCount)
					}
				}
				if ci.HasActions {
					actionsIcon = IconOK
					actionsSev = SeverityOK
				}
			}

			// Overall severity: OK if both or at least Prow, warning if partial.
			overallSev := SeverityOK
			if prowSev == SeverityIssue && actionsSev == SeverityIssue {
				overallSev = SeverityIssue
			} else if prowSev == SeverityIssue || actionsSev == SeverityIssue {
				overallSev = SeverityWarning
			}

			// Expand/collapse indicator for repos with jobs.
			repoArrow := " "
			hasJobs := ci != nil && (len(ci.ProwJobs) > 0 || len(ci.ActionsWflows) > 0 || len(ci.CIOperatorBranches) > 0)
			if hasJobs {
				if m.expanded[repo.Name] {
					repoArrow = IconExpanded
				} else {
					repoArrow = IconCollapsed
				}
			}

			summaryText := fmt.Sprintf("  %s %-40s Go %-5s  %s %s  Actions %s",
				repoArrow, repo.Name, goVer, prowLabel, prowIcon, actionsIcon)

			// URL to the repo on GitHub (or ci-operator config dir if no explicit jobs).
			repoURL := fmt.Sprintf("https://github.com/%s", repo.Name)
			if ci != nil && ci.HasCIOperator && len(ci.ProwJobs) == 0 {
				parts := strings.SplitN(repo.Name, "/", 2)
				if len(parts) == 2 {
					repoURL = fmt.Sprintf("https://github.com/openshift/release/tree/main/ci-operator/config/%s/%s", parts[0], parts[1])
				}
			}

			m.rows = append(m.rows, ciRow{
				kind:     rowRepoSummary,
				groupIdx: gi,
				repoName: repo.Name,
				text:     summaryText,
				severity: overallSev,
				url:      repoURL,
			})

			// Expanded job detail rows.
			if m.expanded[repo.Name] && ci != nil {
				// ci-operator branches (when no explicit Prow jobs).
				if len(ci.ProwJobs) == 0 && len(ci.CIOperatorBranches) > 0 {
					for _, br := range ci.CIOperatorBranches {
						bKey := repo.Name + ":" + br.Branch
						brArrow := IconCollapsed
						if m.expandedBranches[bKey] {
							brArrow = IconExpanded
						}

						testLabel := "tests"
						if len(br.Tests) == 1 {
							testLabel = "test"
						}
						brText := fmt.Sprintf("    %s %s (%d %s)", brArrow, br.Branch, len(br.Tests), testLabel)

						brURL := fmt.Sprintf("https://github.com/%s/tree/%s", repo.Name, br.Branch)

						m.rows = append(m.rows, ciRow{
							kind:      rowBranchHeader,
							groupIdx:  gi,
							repoName:  repo.Name,
							branchKey: bKey,
							text:      brText,
							severity:  SeverityOK,
							url:       brURL,
						})

						// Show test names when branch is expanded.
						if m.expandedBranches[bKey] {
							for _, t := range br.Tests {
								testText := fmt.Sprintf("        %s %s", IconOK, t.Name)
								m.rows = append(m.rows, ciRow{
									kind:      rowRepoDetail,
									groupIdx:  gi,
									repoName:  repo.Name,
									branchKey: bKey,
									text:      testText,
									severity:  SeverityOK,
									url:       brURL,
								})
							}
						}
					}
				}

				// Prow jobs.
				for _, job := range ci.ProwJobs {
					jobIcon := IconOK
					detail := job.Type
					if job.RunIfChanged != "" {
						detail += " (run_if_changed)"
					} else if job.AlwaysRun {
						detail += " (always_run)"
					}

					parts := strings.SplitN(repo.Name, "/", 2)
					jobURL := ""
					if len(parts) == 2 {
						jobURL = fmt.Sprintf("https://github.com/openshift/release/tree/main/core-services/prow/02_config/%s/%s/_prowconfig.yaml", parts[0], parts[1])
					}

					jobText := fmt.Sprintf("    %s %-45s %s", jobIcon, job.Name, detail)
					m.rows = append(m.rows, ciRow{
						kind:     rowRepoDetail,
						groupIdx: gi,
						repoName: repo.Name,
						text:     jobText,
						severity: SeverityOK,
						url:      jobURL,
					})
				}

				// GitHub Actions workflows.
				for _, wf := range ci.ActionsWflows {
					triggers := strings.Join(wf.Triggers, ", ")
					if triggers == "" {
						triggers = "unknown"
					}

					wfURL := fmt.Sprintf("https://github.com/%s/blob/HEAD/.github/workflows/%s",
						repo.Name, wf.Filename)

					displayName := wf.Filename
					if wf.Name != "" && wf.Name != wf.Filename {
						displayName = wf.Filename + " (" + wf.Name + ")"
					}

					wfText := fmt.Sprintf("    %s %-45s %s", IconOK, displayName, triggers)
					m.rows = append(m.rows, ciRow{
						kind:     rowRepoDetail,
						groupIdx: gi,
						repoName: repo.Name,
						text:     wfText,
						severity: SeverityInfo,
						url:      wfURL,
					})
				}
			}
		}
	}

	m.allCIRows = make([]ciRow, len(m.rows))
	copy(m.allCIRows, m.rows)
	m.applyFilter(m.filterQuery)
}

func (m *ciModel) applyFilter(query string) {
	m.filterQuery = query
	if query == "" {
		m.rows = m.allCIRows
		m.clampCursor()
		return
	}
	var filtered []ciRow
	groupHasMatch := make(map[int]bool)
	for _, row := range m.allCIRows {
		if row.kind != rowGroupHeader && matchesFilter(row.text, query) {
			groupHasMatch[row.groupIdx] = true
		}
	}
	for _, row := range m.allCIRows {
		if row.kind == rowGroupHeader {
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

func (m ciModel) View(width, height int, filterQuery string) string {
	if m.loading {
		return RowDimStyle.Render("  Loading CI data...")
	}
	if len(m.rows) == 0 {
		return RowDimStyle.Render("  No CI data available.")
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
		} else {
			hl := highlightMatch(line, filterQuery)
			switch row.kind {
			case rowGroupHeader:
				styled = SectionHeaderStyle.Render(hl)
			case rowGroupDesc:
				styled = RowDimStyle.Render(hl)
			case rowBranchHeader:
				styled = SeverityStyle(row.severity).Render(hl)
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

func (m *ciModel) Update(msg tea.KeyMsg) tea.Cmd {
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
				ci := m.ciInfos[row.repoName]
				hasJobs := ci != nil && (len(ci.ProwJobs) > 0 || len(ci.ActionsWflows) > 0 || len(ci.CIOperatorBranches) > 0)
				if hasJobs {
					m.expanded[row.repoName] = !m.expanded[row.repoName]
					m.buildRows()
				} else if row.url != "" {
					openBrowser(row.url)
				}
			case rowBranchHeader:
				if row.branchKey != "" {
					if m.expandedBranches[row.branchKey] {
						// Already expanded — open branch URL
						if row.url != "" {
							openBrowser(row.url)
						}
					} else {
						m.expandedBranches[row.branchKey] = true
						m.buildRows()
					}
				}
			}
		}
	case "left", "h":
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
			case rowBranchHeader:
				if row.branchKey != "" && m.expandedBranches[row.branchKey] {
					m.expandedBranches[row.branchKey] = false
					m.buildRows()
				}
			case rowRepoDetail:
				// Collapse parent branch if this is a branch test row.
				if row.branchKey != "" && m.expandedBranches[row.branchKey] {
					m.expandedBranches[row.branchKey] = false
					m.buildRows()
				} else if row.repoName != "" && m.expanded[row.repoName] {
					m.expanded[row.repoName] = false
					m.buildRows()
				}
			}
		}
	case "right", "l":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			switch row.kind {
			case rowGroupHeader:
				if m.collapsed[row.groupIdx] {
					m.collapsed[row.groupIdx] = false
					m.buildRows()
				}
			case rowRepoSummary:
				ci := m.ciInfos[row.repoName]
				hasJobs := ci != nil && (len(ci.ProwJobs) > 0 || len(ci.ActionsWflows) > 0 || len(ci.CIOperatorBranches) > 0)
				if hasJobs {
					if !m.expanded[row.repoName] {
						m.expanded[row.repoName] = true
						m.buildRows()
					}
				} else if row.url != "" {
					openBrowser(row.url)
				}
			case rowBranchHeader:
				if row.branchKey != "" {
					if !m.expandedBranches[row.branchKey] {
						m.expandedBranches[row.branchKey] = true
						m.buildRows()
					} else if row.url != "" {
						openBrowser(row.url)
					}
				}
			}
		}
	case "o":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			if u := m.rows[m.cursor].url; u != "" {
				openBrowser(u)
			}
		}
	case "e":
		for gi := range m.groups {
			m.collapsed[gi] = false
		}
		m.buildRows()
	case "c":
		for gi := range m.groups {
			m.collapsed[gi] = true
		}
		m.buildRows()
	case "y":
		// Copy current row text (no-op without clipboard integration, but follows pattern).
		// Could integrate with clipboard in the future.
	}

	m.clampCursor()
	return nil
}

func (m *ciModel) clampCursor() {
	m.cursor, m.scrollOff = clampScroll(m.cursor, m.scrollOff, len(m.rows))
}

// CursorRepo returns the repo name at the cursor, or "".
func (m ciModel) CursorRepo() string {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].repoName
	}
	return ""
}
