package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const doubleClickThreshold = 400 * time.Millisecond

// copyFlashMsg clears the "Copied!" flash after a delay.
type copyFlashMsg struct{}

type model struct {
	width, height int
	activeTab     int // 0=audit, 1=queue, 2=compare, 3=tide, 4=ci

	report  *AuditReport
	configs map[string]*RepoConfig
	loading bool
	err     error
	flash   string // brief status message (e.g. "Copied!")

	auditTab   auditModel
	queueTab   queueModel
	compareTab compareModel
	tideTab    tideModel
	ciTab      ciModel

	help        helpModel
	spinner     spinner.Model
	searching   bool
	searchInput textinput.Model
	searchQuery [5]string // per-tab filter query

	branch      string
	localPath   string
	token       string
	skipQueue   bool
	ghAvailable bool

	// Tab bar click zones: tabZones[i] = {startCol, endCol} for tab i.
	tabZones [5][2]int

	// Previous audit findings for diff-on-refresh.
	prevFindings map[string][]Finding

	// Double-click tracking.
	lastClickTime time.Time
	lastClickRow  int
	lastClickRepo string

	// Watch mode (auto-refresh).
	watchInterval time.Duration
	watchPaused   bool
}

// Message types.
type auditDoneMsg struct {
	report  *AuditReport
	configs map[string]*RepoConfig
}

type queueDoneMsg struct {
	repo   string
	report *MergeQueueReport
}

type errMsg struct {
	err error
}

type watchTickMsg struct{}

func watchTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return watchTickMsg{}
	})
}

var tabNames = []string{"Config Audit", "Merge Queue", "Field Compare", "Tide Branches", "CI"}

func newModel(branch, localPath, token string, skipQueue, ghAvailable bool, watchInterval time.Duration) model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 40

	_, zones := renderTabBar(0, 200)
	return model{
		spinner:     s,
		searchInput: ti,
		loading:     true,
		branch:      branch,
		localPath:   localPath,
		token:       token,
		skipQueue:   skipQueue,
		ghAvailable:   ghAvailable,
		watchInterval: watchInterval,
		queueTab:      newQueueModel(),
		help:          helpModel{},
		tabZones:      zones,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		runAuditCmd(m.branch, m.localPath, m.token),
	}
	if m.watchInterval > 0 {
		cmds = append(cmds, watchTickCmd(m.watchInterval))
	}
	return tea.Batch(cmds...)
}

func runAuditCmd(branch, localPath, token string) tea.Cmd {
	return func() tea.Msg {
		report, configs, err := RunAudit(branch, localPath, token)
		if err != nil {
			return errMsg{err: err}
		}
		return auditDoneMsg{report: report, configs: configs}
	}
}

// queueConfigFromAudit builds a QueueConfig from audit report data.
func queueConfigFromAudit(report *AuditReport, repoName string) *QueueConfig {
	if report == nil {
		return &QueueConfig{}
	}
	for _, g := range report.Groups {
		for _, r := range g.Repos {
			if r.Name == repoName {
				qc := &QueueConfig{}
				if v, ok := r.Fields["required_approving_review_count"]; ok && v != "NOT_SET" {
					_, _ = fmt.Sscanf(v, "%d", &qc.RequiredReviews)
				}
				if v, ok := r.Fields["enforce_admins"]; ok && v == "true" {
					qc.EnforceAdmins = true
				}
				return qc
			}
		}
	}
	return &QueueConfig{}
}

func runQueueCmd(repo string, qc *QueueConfig) tea.Cmd {
	return func() tea.Msg {
		report, err := CheckMergeQueue(repo, qc)
		if err != nil {
			return queueDoneMsg{repo: repo, report: &MergeQueueReport{Repo: repo, Error: err.Error()}}
		}
		return queueDoneMsg{repo: repo, report: report}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recompute tab click zones on resize.
		_, m.tabZones = renderTabBar(m.activeTab, m.width)
		return m, nil

	case tea.MouseMsg:
		if m.help.visible {
			return m, nil
		}
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}

		row := msg.Y
		col := msg.X

		switch {
		// Row 0 = header, Row 1 = tab bar.
		case row == 1 && msg.Button == tea.MouseButtonLeft:
			for i, zone := range m.tabZones {
				if col >= zone[0] && col < zone[1] {
					m.activeTab = i
					return m, nil
				}
			}

		// Body area clicks (row >= 2, before footer).
		case row >= 2 && row < m.height-1 && msg.Button == tea.MouseButtonLeft:
			bodyRow := row - 2 // offset for header + tab bar
			now := time.Now()

			// Check globe 🌐 click first — opens browser immediately.
			{
				var globeURL string
				var globeTextWidth int
				switch m.activeTab {
				case 0:
					idx := m.auditTab.scrollOff + bodyRow
					if idx >= 0 && idx < len(m.auditTab.rows) && m.auditTab.rows[idx].url != "" {
						globeURL = m.auditTab.rows[idx].url
						globeTextWidth = lipgloss.Width(m.auditTab.rows[idx].text)
					}
				case 1:
					idx := m.queueTab.scrollOff + bodyRow - 1
					if idx >= 0 && idx < len(m.queueTab.rows) && m.queueTab.rows[idx].url != "" {
						globeURL = m.queueTab.rows[idx].url
						globeTextWidth = lipgloss.Width(m.queueTab.rows[idx].text)
					}
				case 3:
					idx := m.tideTab.scrollOff + bodyRow
					if idx >= 0 && idx < len(m.tideTab.rows) && m.tideTab.rows[idx].url != "" {
						globeURL = m.tideTab.rows[idx].url
						globeTextWidth = lipgloss.Width(m.tideTab.rows[idx].text)
					}
				case 4:
					idx := m.ciTab.scrollOff + bodyRow
					if idx >= 0 && idx < len(m.ciTab.rows) && m.ciTab.rows[idx].url != "" {
						globeURL = m.ciTab.rows[idx].url
						globeTextWidth = lipgloss.Width(m.ciTab.rows[idx].text)
					}
				}
				// Globe 🌐 is at columns [textWidth+1, textWidth+3) — space + 2-wide emoji.
				if globeURL != "" && col >= globeTextWidth+1 && col <= globeTextWidth+3 {
					openBrowser(globeURL)
					return m, nil
				}
			}

			// Check [m] button click in audit tab.
			if m.activeTab == 0 && m.report != nil {
				if btnRepo := m.auditTab.ClickedMButton(bodyRow, col); btnRepo != "" {
					if !m.ghAvailable {
						m.err = fmt.Errorf("merge queue requires gh CLI — install from https://cli.github.com/")
						return m, nil
					}
					rc := queueConfigFromAudit(m.report, btnRepo)
					m.queueTab.loading[btnRepo] = true
					m.activeTab = 1
					return m, runQueueCmd(btnRepo, rc)
				}
			}

			// Resolve clicked row info based on active tab.
			clickedRepo := ""
			clickedURL := ""
			switch m.activeTab {
			case 0:
				targetIdx := m.auditTab.scrollOff + bodyRow
				if targetIdx >= 0 && targetIdx < len(m.auditTab.rows) {
					m.auditTab.cursor = targetIdx
					m.auditTab.clampCursor()
					row := m.auditTab.rows[targetIdx]
					clickedRepo = row.repoName
					clickedURL = row.url
					// Click on group header toggles collapse.
					if row.kind == rowGroupHeader {
						m.auditTab.collapsed[row.groupIdx] = !m.auditTab.collapsed[row.groupIdx]
						m.auditTab.buildRows()
						m.auditTab.clampCursor()
						return m, nil
					}
					// Click on repo summary toggles detail expand.
					if row.kind == rowRepoSummary {
						m.auditTab.expanded[row.repoName] = !m.auditTab.expanded[row.repoName]
						m.auditTab.buildRows()
						m.auditTab.clampCursor()
						return m, nil
					}
				}
			case 1:
				targetIdx := m.queueTab.scrollOff + bodyRow - 1 // -1 for sort mode indicator line
				if targetIdx >= 0 && targetIdx < len(m.queueTab.rows) {
					m.queueTab.cursor = targetIdx
					m.queueTab.clampCursor()
					qRow := m.queueTab.rows[targetIdx]
					clickedRepo = qRow.repoName
					clickedURL = qRow.url
					// Click on action button triggers check-all.
					if qRow.kind == rowAction && m.report != nil {
						if !m.ghAvailable {
							m.err = fmt.Errorf("check-all requires gh CLI — install from https://cli.github.com/")
							return m, nil
						}
						if m.token == "" {
							m.err = fmt.Errorf("check-all requires authentication — run 'gh auth login' or set GITHUB_TOKEN (use 'm' for single repo)")
							return m, nil
						}
						if rl := fetchRateLimitFn(); rl != nil && rl.Remaining < 100 {
							m.flash = fmt.Sprintf("%s API rate limit low: %d/%d remaining (resets %s)", IconWarn, rl.Remaining, rl.Limit, rl.ResetAt.Local().Format("15:04"))
						}
						var cmds []tea.Cmd
						for _, g := range m.report.Groups {
							for _, r := range g.Repos {
								rc := queueConfigFromAudit(m.report, r.Name)
								m.queueTab.loading[r.Name] = true
								cmds = append(cmds, runQueueCmd(r.Name, rc))
							}
						}
						return m, tea.Batch(cmds...)
					}
					// Click on [m] prefix of repo header → check that repo.
					if qRow.kind == rowGroupHeader && qRow.repoName != "" && col < 4 && m.report != nil {
						if !m.ghAvailable {
							m.err = fmt.Errorf("merge queue requires gh CLI — install from https://cli.github.com/")
							return m, nil
						}
						rc := queueConfigFromAudit(m.report, qRow.repoName)
						m.queueTab.loading[qRow.repoName] = true
						m.queueTab.buildRows()
						return m, runQueueCmd(qRow.repoName, rc)
					}
					// Click elsewhere on repo header toggles collapse.
					if qRow.kind == rowGroupHeader && qRow.repoName != "" {
						m.queueTab.collapsed[qRow.repoName] = !m.queueTab.collapsed[qRow.repoName]
						m.queueTab.buildRows()
						m.queueTab.clampCursor()
						return m, nil
					}
					// Click on PR summary toggles PR detail collapse.
					if qRow.kind == rowRepoSummary && qRow.prKey != "" {
						m.queueTab.prCollapsed[qRow.prKey] = !m.queueTab.prCollapsed[qRow.prKey]
						m.queueTab.buildRows()
						m.queueTab.clampCursor()
						return m, nil
					}
				}
			case 2:
				targetIdx := m.compareTab.scrollOff + bodyRow - 1 // -1 for mode indicator line
				if targetIdx >= 0 && targetIdx < len(m.compareTab.rows) {
					m.compareTab.cursor = targetIdx
					m.compareTab.clampCursor()
				}
			case 3:
				targetIdx := m.tideTab.scrollOff + bodyRow
				if targetIdx >= 0 && targetIdx < len(m.tideTab.rows) {
					m.tideTab.cursor = targetIdx
					clickedURL = m.tideTab.rows[targetIdx].url
					m.tideTab.clampCursor()
				}
			case 4:
				targetIdx := m.ciTab.scrollOff + bodyRow
				if targetIdx >= 0 && targetIdx < len(m.ciTab.rows) {
					m.ciTab.cursor = targetIdx
					m.ciTab.clampCursor()
					row := m.ciTab.rows[targetIdx]
					clickedRepo = row.repoName
					clickedURL = row.url
					if row.kind == rowGroupHeader {
						m.ciTab.collapsed[row.groupIdx] = !m.ciTab.collapsed[row.groupIdx]
						m.ciTab.buildRows()
						m.ciTab.clampCursor()
						return m, nil
					}
					if row.kind == rowRepoSummary {
						m.ciTab.expanded[row.repoName] = !m.ciTab.expanded[row.repoName]
						m.ciTab.buildRows()
						m.ciTab.clampCursor()
						return m, nil
					}
				}
			}

			// Double-click detection.
			isDoubleClick := row == m.lastClickRow &&
				now.Sub(m.lastClickTime) < doubleClickThreshold

			if isDoubleClick {
				m.lastClickRepo = ""
				m.lastClickTime = time.Time{}

				// If the row has a URL, open it in browser.
				if clickedURL != "" {
					openBrowser(clickedURL)
					return m, nil
				}

				// Otherwise refresh the repo (audit tab behavior).
				if clickedRepo != "" {
					ClearConfigCache(clickedRepo)
					m.loading = true
					return m, runAuditCmd(m.branch, m.localPath, m.token)
				}
				return m, nil
			}

			m.lastClickTime = now
			m.lastClickRow = row
			m.lastClickRepo = clickedRepo
			return m, nil

		// Scroll wheel.
		case msg.Button == tea.MouseButtonWheelUp:
			switch m.activeTab {
			case 0:
				m.auditTab.cursor--
				m.auditTab.clampCursor()
			case 1:
				m.queueTab.cursor--
				m.queueTab.clampCursor()
			case 2:
				m.compareTab.cursor--
				m.compareTab.clampCursor()
			case 3:
				m.tideTab.cursor--
				m.tideTab.clampCursor()
			case 4:
				m.ciTab.cursor--
				m.ciTab.clampCursor()
			}
			return m, nil

		case msg.Button == tea.MouseButtonWheelDown:
			switch m.activeTab {
			case 0:
				m.auditTab.cursor++
				m.auditTab.clampCursor()
			case 1:
				m.queueTab.cursor++
				m.queueTab.clampCursor()
			case 2:
				m.compareTab.cursor++
				m.compareTab.clampCursor()
			case 3:
				m.tideTab.cursor++
				m.tideTab.clampCursor()
			case 4:
				m.ciTab.cursor++
				m.ciTab.clampCursor()
			}
			return m, nil
		}

		return m, nil

	case tea.KeyMsg:
		// Handle help overlay first.
		if m.help.visible {
			switch msg.String() {
			case "?", "esc":
				m.help.visible = false
			}
			return m, nil
		}

		// Handle search input mode.
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchQuery[m.activeTab] = ""
				m.searchInput.Reset()
				m.applyFilterToActiveTab("")
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				q := m.searchInput.Value()
				m.searchQuery[m.activeTab] = q
				m.applyFilterToActiveTab(q)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help.visible = true
			return m, nil
		case "esc":
			// Clear active filter if present.
			if m.searchQuery[m.activeTab] != "" {
				m.searchQuery[m.activeTab] = ""
				m.searchInput.Reset()
				m.applyFilterToActiveTab("")
			}
			return m, nil
		case "/":
			m.searching = true
			m.searchInput.SetValue(m.searchQuery[m.activeTab])
			m.searchInput.Focus()
			return m, m.searchInput.Cursor.BlinkCmd()
		case "r":
			m.loading = true
			ClearConfigCache("")
			cmds := []tea.Cmd{runAuditCmd(m.branch, m.localPath, m.token)}
			if m.watchInterval > 0 {
				cmds = append(cmds, watchTickCmd(m.watchInterval))
			}
			return m, tea.Batch(cmds...)
		case "p":
			if m.watchInterval > 0 {
				m.watchPaused = !m.watchPaused
				if m.watchPaused {
					m.flash = "Watch paused"
				} else {
					m.flash = "Watch resumed"
					return m, tea.Batch(
						tea.Tick(2*time.Second, func(time.Time) tea.Msg { return copyFlashMsg{} }),
						watchTickCmd(m.watchInterval),
					)
				}
				return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return copyFlashMsg{} })
			}
		case "1":
			m.activeTab = 0
			return m, nil
		case "2":
			m.activeTab = 1
			return m, nil
		case "3":
			m.activeTab = 2
			return m, nil
		case "4":
			m.activeTab = 3
			return m, nil
		case "5":
			m.activeTab = 4
			return m, nil
		case "tab", "shift+right":
			m.activeTab = (m.activeTab + 1) % 5
			return m, nil
		case "shift+tab", "shift+left":
			m.activeTab = (m.activeTab + 4) % 5
			return m, nil
		case "m":
			if !m.ghAvailable {
				m.err = fmt.Errorf("merge queue requires gh CLI — install from https://cli.github.com/")
				return m, nil
			}
			if m.report != nil {
				var repo string
				switch m.activeTab {
				case 0:
					repo = m.auditTab.CursorRepo()
				case 1:
					if m.queueTab.cursor >= 0 && m.queueTab.cursor < len(m.queueTab.rows) {
						repo = m.queueTab.rows[m.queueTab.cursor].repoName
					}
				}
				if repo != "" {
					m.err = nil // clear any previous error
					rc := queueConfigFromAudit(m.report, repo)
					m.queueTab.loading[repo] = true
					m.activeTab = 1
					return m, runQueueCmd(repo, rc)
				}
			}
		case "c":
			if m.activeTab == 0 && m.report != nil {
				suggestion := m.auditTab.CursorSuggestion()
				if suggestion != "" {
					if err := copyToClipboard(suggestion); err != nil {
						m.err = fmt.Errorf("copy failed: %v", err)
					} else {
						m.flash = "Copied suggestion!"
						return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
							return copyFlashMsg{}
						})
					}
				}
			}
			return m, nil
		case "M":
			if !m.ghAvailable {
				m.err = fmt.Errorf("check-all requires gh CLI — install from https://cli.github.com/")
				return m, nil
			}
			if m.token == "" {
				m.err = fmt.Errorf("check-all requires authentication — run 'gh auth login' or set GITHUB_TOKEN (use 'm' for single repo)")
				return m, nil
			}
			if m.report != nil {
				if rl := fetchRateLimitFn(); rl != nil && rl.Remaining < 100 {
					m.flash = fmt.Sprintf("%s API rate limit low: %d/%d remaining (resets %s)", IconWarn, rl.Remaining, rl.Limit, rl.ResetAt.Local().Format("15:04"))
				}
				var cmds []tea.Cmd
				for _, g := range m.report.Groups {
					for _, r := range g.Repos {
						rc := queueConfigFromAudit(m.report, r.Name)
						m.queueTab.loading[r.Name] = true
						cmds = append(cmds, runQueueCmd(r.Name, rc))
					}
				}
				m.activeTab = 1
				return m, tea.Batch(cmds...)
			}
		case "y":
			if m.report == nil {
				return m, nil
			}
			text := m.copyText()
			if text != "" {
				if err := copyToClipboard(text); err != nil {
					m.err = fmt.Errorf("copy failed: %v", err)
				} else {
					m.flash = "Copied!"
					return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
						return copyFlashMsg{}
					})
				}
			}
			return m, nil
		default:
			// Intercept Enter on queue tab action rows.
			if m.activeTab == 1 && (msg.String() == "enter" || msg.String() == " ") {
				if m.queueTab.cursor >= 0 && m.queueTab.cursor < len(m.queueTab.rows) {
					row := m.queueTab.rows[m.queueTab.cursor]
					if row.kind == rowAction && m.report != nil {
						// "Check All" action — apply same guards as keyboard 'M'.
						if !m.ghAvailable {
							m.err = fmt.Errorf("check-all requires gh CLI — install from https://cli.github.com/")
							return m, nil
						}
						if m.token == "" {
							m.err = fmt.Errorf("check-all requires authentication — run 'gh auth login' or set GITHUB_TOKEN (use 'm' for single repo)")
							return m, nil
						}
						if rl := fetchRateLimitFn(); rl != nil && rl.Remaining < 100 {
							m.flash = fmt.Sprintf("%s API rate limit low: %d/%d remaining (resets %s)", IconWarn, rl.Remaining, rl.Limit, rl.ResetAt.Local().Format("15:04"))
						}
						var cmds []tea.Cmd
						for _, g := range m.report.Groups {
							for _, r := range g.Repos {
								rc := queueConfigFromAudit(m.report, r.Name)
								m.queueTab.loading[r.Name] = true
								cmds = append(cmds, runQueueCmd(r.Name, rc))
							}
						}
						return m, tea.Batch(cmds...)
					}
				}
			}

			// Delegate to active tab.
			var cmd tea.Cmd
			switch m.activeTab {
			case 0:
				cmd = m.auditTab.Update(msg)
			case 1:
				cmd = m.queueTab.Update(msg)
			case 2:
				cmd = m.compareTab.Update(msg)
			case 3:
				cmd = m.tideTab.Update(msg)
			case 4:
				cmd = m.ciTab.Update(msg)
			}
			return m, cmd
		}

	case watchTickMsg:
		if m.watchInterval <= 0 {
			return m, nil
		}
		if m.watchPaused || m.loading {
			return m, watchTickCmd(m.watchInterval)
		}
		m.loading = true
		ClearConfigCache("")
		return m, tea.Batch(
			runAuditCmd(m.branch, m.localPath, m.token),
			watchTickCmd(m.watchInterval),
		)

	case auditDoneMsg:
		// Snapshot current findings before overwriting for diff-on-refresh.
		var prevRateLimit *RateLimit
		if m.report != nil {
			m.prevFindings = snapshotFindings(m.report)
			prevRateLimit = m.report.RateLimit
		}
		m.report = msg.report
		// Preserve previous rate limit if new fetch returned nil.
		if m.report != nil && m.report.RateLimit == nil && prevRateLimit != nil {
			m.report.RateLimit = prevRateLimit
		}
		m.configs = msg.configs
		m.loading = false
		m.err = nil
		var cmds []tea.Cmd
		if m.report != nil {
			// Save view state before replacing audit model.
			var savedState *auditViewState
			if m.auditTab.groups != nil {
				s := m.auditTab.saveViewState()
				savedState = &s
			}
			m.auditTab = newAuditModel(m.report.Groups, m.configs, m.prevFindings)
			// Restore view state after creating new model.
			if savedState != nil {
				m.auditTab.restoreViewState(*savedState)
			}
			// Set flash with diff summary.
			if m.prevFindings != nil {
				if flash := diffSummaryFlash(m.report, m.prevFindings); flash != "" {
					m.flash = flash
				}
			}
			m.compareTab = newCompareModel(m.report)
			m.tideTab = newTideModel(m.report.Groups)
			// Initialize CI tab with loading state.
			m.ciTab = newCIModel(m.report.Groups, nil)
			m.ciTab.loading = true
			// Populate queue tab's repo list.
			var allRepos []string
			for _, g := range m.report.Groups {
				for _, r := range g.Repos {
					allRepos = append(allRepos, r.Name)
				}
			}
			m.queueTab.allRepos = allRepos
			m.queueTab.ghAvailable = m.ghAvailable
			m.queueTab.buildRows()
			// Check rate limit before batch CI fetch.
			if rl := fetchRateLimitFn(); rl != nil && rl.Remaining < 100 {
				m.flash = fmt.Sprintf("%s API rate limit low: %d/%d remaining (resets %s)", IconWarn, rl.Remaining, rl.Limit, rl.ResetAt.Local().Format("15:04"))
			}
			// Start async CI data fetch.
			configs := m.configs
			token := m.token
			localPath := m.localPath
			branch := m.branch
			cmds = append(cmds, func() tea.Msg {
				ciInfos := FetchAllCIInfo(allRepos, configs, localPath, branch, token, 30*time.Second)
				return ciDoneMsg{ciInfos: ciInfos}
			})
		}
		return m, tea.Batch(cmds...)

	case ciDoneMsg:
		if m.report != nil {
			m.ciTab = newCIModel(m.report.Groups, msg.ciInfos)
			// Re-fetch rate limit after CI data fetch (uses API quota).
			if rl := fetchRateLimitFn(); rl != nil {
				m.report.RateLimit = rl
			}
		}
		return m, nil

	case queueDoneMsg:
		m.queueTab.SetQueue(msg.repo, msg.report)
		// Re-fetch rate limit after queue check (gh CLI uses API quota).
		if rl := fetchRateLimitFn(); rl != nil && m.report != nil {
			m.report.RateLimit = rl
		}
		return m, nil

	case copyFlashMsg:
		m.flash = ""
		return m, nil

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case spinner.TickMsg:
		if m.loading || len(m.queueTab.loading) > 0 {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	// Full-screen loading state before any data.
	if m.loading && m.report == nil {
		loadingMsg := fmt.Sprintf("Loading... %s", m.spinner.View())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, loadingMsg)
	}

	// Compose layout: header + tab bar + body + footer.
	header := renderHeaderWithQueue(m.width, m.report, m.loading, m.spinner.View(), m.queueTab.loading, m.watchInterval, m.watchPaused)
	tabs, _ := renderTabBar(m.activeTab, m.width)

	cursorRepo := ""
	onHeader := false
	if m.activeTab == 0 {
		cursorRepo = m.auditTab.CursorRepo()
		onHeader = m.auditTab.CursorOnGroup()
	}
	hasFilter := m.searchQuery[m.activeTab] != ""
	var footer string
	if m.searching {
		footer = renderSearchFooter(m.width, m.searchInput)
	} else {
		footer = renderFooter(m.width, m.activeTab, onHeader, hasFilter, cursorRepo, m.watchInterval > 0)
	}

	// Body height: total - header(1) - tabs(1) - footer(1).
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	fq := m.searchQuery[m.activeTab]
	var body string
	switch m.activeTab {
	case 0:
		body = m.auditTab.View(m.width, bodyHeight, fq)
	case 1:
		body = m.queueTab.View(m.width, bodyHeight, fq)
	case 2:
		body = m.compareTab.View(m.width, bodyHeight, fq)
	case 3:
		body = m.tideTab.View(m.width, bodyHeight, fq)
	case 4:
		body = m.ciTab.View(m.width, bodyHeight, fq)
	}

	// Ensure body fills the height.
	bodyLines := strings.Count(body, "\n") + 1
	for bodyLines < bodyHeight {
		body += "\n"
		bodyLines++
	}

	// Flash message display.
	if m.flash != "" {
		flashLine := RowOKStyle.Render(m.flash)
		body = flashLine + "\n" + body
	} else if m.err != nil {
		errLine := RowIssueStyle.Render(fmt.Sprintf("Error: %v", m.err))
		body = errLine + "\n" + body
	}

	screen := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)

	// Help overlay.
	if m.help.visible {
		screen = m.help.View(m.width, m.height)
	}

	return screen
}

// copyText returns the text to copy based on current tab and cursor position.
// On a repo row: copies that repo's details. On a group header: copies the group.
// Otherwise: copies all results for the current tab.
func (m model) copyText() string {
	switch m.activeTab {
	case 0: // audit
		if m.auditTab.cursor >= 0 && m.auditTab.cursor < len(m.auditTab.rows) {
			row := m.auditTab.rows[m.auditTab.cursor]
			if row.repoName != "" {
				return RenderRepoText(m.report, row.repoName)
			}
			if row.kind == rowGroupHeader {
				return RenderGroupText(m.report, row.groupIdx)
			}
		}
		// Fallback: full text report
		return RenderTextPlain(m.report)
	case 1: // queue
		if m.queueTab.cursor >= 0 && m.queueTab.cursor < len(m.queueTab.rows) {
			row := m.queueTab.rows[m.queueTab.cursor]
			if row.repoName != "" {
				return m.queueTab.copyRepo(row.repoName)
			}
		}
		return m.queueTab.copyAll()
	case 2: // compare
		if m.compareTab.cursor >= 0 && m.compareTab.cursor < len(m.compareTab.rows) {
			row := m.compareTab.rows[m.compareTab.cursor]
			if row.isHeader {
				return m.compareTab.copyGroup(row.groupIdx)
			}
		}
		return m.compareTab.copyAll()
	case 3: // tide
		if m.tideTab.cursor >= 0 && m.tideTab.cursor < len(m.tideTab.rows) {
			row := m.tideTab.rows[m.tideTab.cursor]
			if row.isHeader {
				return m.tideTab.copyGroup(row.groupIdx)
			}
		}
		return m.tideTab.copyAll()
	}
	return ""
}

// applyFilterToActiveTab applies the given filter query to the currently active tab.
func (m *model) applyFilterToActiveTab(query string) {
	switch m.activeTab {
	case 0:
		m.auditTab.applyFilter(query)
	case 1:
		m.queueTab.applyFilter(query)
	case 2:
		m.compareTab.applyFilter(query)
	case 3:
		m.tideTab.applyFilter(query)
	case 4:
		m.ciTab.applyFilter(query)
	}
}

func renderTabBar(activeTab, width int) (string, [5][2]int) {
	var zones [5][2]int
	var parts []string
	col := 0
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		plainWidth := lipgloss.Width(label)
		zones[i] = [2]int{col, col + plainWidth}
		if i == activeTab {
			parts = append(parts, TabActiveStyle.Render(label))
		} else {
			parts = append(parts, TabInactiveStyle.Render(label))
		}
		col += plainWidth + 2 // +2 for "  " separator
	}

	bar := strings.Join(parts, "  ")

	barWidth := lipgloss.Width(bar)
	if barWidth < width {
		bar += strings.Repeat(" ", width-barWidth)
	}

	return bar, zones
}
