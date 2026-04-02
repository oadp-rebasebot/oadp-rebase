# Prow Audit TUI — Go Implementation Design

## Overview

Replace the bash `tui_viewer()` in `audit.sh` with a Go TUI built on [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles). The bash script continues to serve as the data-collection engine (via `--format json`), while Go handles all interactive rendering.

### Why

The bash TUI is at ~80% of what pure bash can achieve. The remaining features — multi-pane layout, interactive tables, fuzzy filtering, diff-based rendering, state preservation across refresh — require disproportionate effort in bash and are trivial with Bubbletea. See [Comparison](#comparison-auditsh-vs-go-tui) below.

### Inspirations

Ranked by relevance to this project:

| Tool | Stars | Key pattern to adopt |
|------|-------|---------------------|
| [gh-dash](https://github.com/dlvhdr/gh-dash) | 11K | Multi-section PR/issue dashboard with filterable tables |
| [k9s](https://github.com/derailed/k9s) | 33K | Row-level status coloring, `:` command mode, Pulse health view |
| [lazygit](https://github.com/jesseduffield/lazygit) | 75K | Master-detail panels, context-sensitive footer keybindings |
| [wtfutil](https://github.com/wtfutil/wtf) | 17K | Modular widget grid with independent refresh intervals |
| [amtui](https://github.com/pehlicd/amtui) | 108 | Alert severity display (maps to pass/fail/warn) |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  audit.sh --format json   (data collection, unchanged)      │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Fetches _pluginconfig.yaml + _prowconfig.yaml           ││
│  │ Runs all audit checks                                   ││
│  │ Checks merge queue via gh CLI                           ││
│  │ Outputs structured JSON to stdout                       ││
│  └─────────────────────────────────────────────────────────┘│
│                            │ JSON                           │
│                            ▼                                │
│  prow-audit-tui            (Go binary, this design)         │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Parses JSON into typed Go structs                       ││
│  │ Renders interactive TUI via Bubbletea                   ││
│  │ Re-invokes audit.sh for refresh / merge queue checks    ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Data flow

1. On startup, the Go binary runs `audit.sh --format json [--local <path>]` as a subprocess
2. JSON is deserialized into `[]RepoAudit` structs
3. Bubbletea renders the TUI from structured data (enables sort, filter, group)
4. User actions like refresh re-invoke the subprocess; merge queue checks invoke `audit.sh --merge-queue <repo> --format json`
5. The TUI state (cursor, scroll, collapsed sections, active tab) is preserved across data refreshes

### Why keep audit.sh as the data engine

- The bash script already handles all the YAML parsing, GitHub API calls, `gh` CLI integration, and audit logic — ~1700 lines of battle-tested code
- Rewriting the data collection in Go would duplicate effort with no UX benefit
- The JSON contract is a clean boundary: audit.sh owns data, Go owns presentation
- Users who prefer the bash TUI or piped text output lose nothing

---

## JSON Contract

### `audit.sh --format json`

Adding a new `--format json` output mode to audit.sh. The schema:

```json
{
  "source": "github.com/openshift/release @ main",
  "timestamp": "2026-04-01T12:00:00Z",
  "rate_limit": {
    "remaining": 4850,
    "limit": 5000,
    "reset_at": "2026-04-01T13:00:00Z"
  },
  "groups": [
    {
      "name": "Upstream Rebase Repos",
      "type": "upstream-rebase",
      "description": "Forks of upstream projects managed by rebasebot",
      "repos": [
        {
          "name": "openshift/velero",
          "has_config": true,
          "fields": {
            "require_self_approval": "false",
            "review_acts_as_lgtm": "NOT_SET",
            "enforce_admins": "NOT_SET",
            "required_approving_review_count": "NOT_SET",
            "allow_force_pushes": "true",
            "dismiss_stale_reviews": "NOT_SET",
            "merge_method": "NOT_SET"
          },
          "plugins": {
            "has_approve_section": true,
            "has_lgtm_section": true,
            "has_approve_plugin": true
          },
          "tide": {
            "included_branches": ["oadp-1.4", "oadp-1.5", "oadp-1.6"],
            "has_keep_main_query_separate": false
          },
          "findings": [
            {
              "severity": "ok",
              "field": "allow_force_pushes",
              "message": "allow_force_pushes=true (expected for upstream rebase repo)"
            },
            {
              "severity": "info",
              "field": "merge_method",
              "message": "merge_method=squash"
            }
          ]
        }
      ]
    }
  ],
  "comparison": {
    "upstream-rebase": {
      "require_self_approval": {
        "majority": "false",
        "outliers": [
          {"repo": "openshift/restic", "value": "NOT_SET"}
        ]
      }
    }
  },
  "summary": {
    "issues": 3,
    "warnings": 5,
    "info": 12
  }
}
```

### `audit.sh --merge-queue <repo> --format json`

```json
{
  "repo": "openshift/oadp-operator",
  "required_reviews": 2,
  "enforce_admins": true,
  "tide_url": "https://prow.ci.openshift.org/tide?query=...",
  "prs": [
    {
      "number": 1234,
      "title": "Fix backup controller",
      "author": "kaovilai",
      "base": "oadp-1.6",
      "head": "fix-backup",
      "approval_count": 1,
      "required_reviews": 2,
      "approving_reviewers": ["user1"],
      "label_blockers": ["needs-rebase"],
      "checks": {
        "failing": ["e2e-aws"],
        "errored": [],
        "pending": ["tide"],
        "in_progress": ["unit-tests"],
        "tide_state": "PENDING"
      },
      "has_blockers": true,
      "review_blocked": true,
      "reviews_short": 1,
      "pr_url": "https://github.com/openshift/oadp-operator/pull/1234",
      "prow_url": "https://prow.ci.openshift.org/pr?query=..."
    }
  ]
}
```

---

## TUI Layout

### Three-tier screen structure

Adopted from the universal pattern across k9s, lazygit, btop, and gh-dash:

```
┌─ Header ──────────────────────────────────────────────────────────────────┐
│ Prow Audit │ openshift/release @ main │ 2026-04-01 12:00Z │ API: 4850/5K │
├─ Tabs ────────────────────────────────────────────────────────────────────┤
│ [1] Config Audit  [2] Merge Queue  [3] Comparison  [4] Tide Branches     │
├─ Body ────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  (tab-specific content — see below)                                      │
│                                                                          │
├─ Footer ──────────────────────────────────────────────────────────────────┤
│ ↑↓ navigate │ Enter expand │ / filter │ Tab next │ r refresh │ ? help    │
└───────────────────────────────────────────────────────────────────────────┘
```

### Header (fixed, 1 line)

- Tool name
- Data source (`openshift/release @ main` or local path)
- Timestamp of last fetch
- API rate limit (color-coded: green >100, yellow <100, red =0 with reset time)
- Loading spinner when a background operation is in progress

### Tab bar (fixed, 1 line)

Number keys `1`-`4` switch tabs. Active tab is highlighted. Each tab renders independently in the body area.

### Footer (fixed, 1-2 lines)

**Context-sensitive** keybinding hints (lazygit pattern). The footer changes based on:
- Which tab is active
- Whether the cursor is on a group header vs a repo row vs a PR
- Whether a filter is active

### Body — Tab 1: Config Audit (default)

The primary view. Repos grouped by type with collapsible group headers.

```
▼ Upstream Rebase Repos (7 repos — allow_force_pushes expected)
  Repos that are forks of upstream projects managed by rebasebot.

  Repo                              Status  Details
  ──────────────────────────────────────────────────────────────
  openshift/velero                  ✓ OK    force_push=true
  openshift/velero-plugin-for-aws   ✓ OK    force_push=true
  openshift/restic                  ⚠ WARN  force_push missing

▼ OADP-Owned Repos — openshift/ (4 repos)
  ...

▶ OADP-Owned Repos — migtools/ (12 repos)  [collapsed]
```

Each repo row is expandable (Enter) to show full details:

```
  openshift/oadp-operator           ⚠ 2 warnings
    ├─ require_self_approval: false
    ├─ review_acts_as_lgtm:  NOT_SET
    ├─ enforce_admins:       true       ✓
    ├─ review_count:         2          ✓
    ├─ dismiss_stale_reviews: true      ✓
    ├─ allow_force_pushes:   NOT_SET    ✓
    ├─ merge_method:         squash     ℹ
    ├─ Plugins: approve ✓  lgtm ✓
    └─ Tide branches: oadp-1.4, oadp-1.5, oadp-1.6
```

Row-level coloring (k9s pattern):
- Green background: all checks OK
- Yellow background: warnings only
- Red background: issues found
- Dim/gray: no config found

### Body — Tab 2: Merge Queue

Loaded on demand (press `2` or `m` on a repo). Shows PRs with approved+lgtm that haven't merged.

```
  openshift/oadp-operator (3 PRs with approved+lgtm)
  Required reviews: 2 │ enforce_admins: true
  Tide: https://prow.ci.openshift.org/tide?query=...

    ✗ #1234 [oadp-1.6] Fix backup controller
      Author: kaovilai
      BLOCKED  label: needs-rebase
      REVIEWS  1/2 GitHub approvals (need 1 more)
      FAILED   e2e-aws
      RUNNING  unit-tests

    ✓ #1235 [oadp-1.6] Update dependencies
      Author: dependabot
      REVIEWS  2/2 GitHub approvals
      TIDE     merge criteria met — merging soon
      READY    All checks passed — merge imminent
```

### Body — Tab 3: Comparison

Interactive table (Bubbles table component) showing cross-repo field comparison.

```
  ── Upstream Rebase Repos ──

  Repo                            self_approve  review_lgtm  force_push  merge
  ────────────────────────────────────────────────────────────────────────────
  openshift/velero                false         NOT_SET      true        squash
  openshift/velero-plugin-aws     false         NOT_SET      true        squash
  openshift/restic                NOT_SET ⚠     NOT_SET      NOT_SET ⚠   squash
                                  (majority:    (majority:   (majority:
                                   false)        NOT_SET)     true)
```

Features:
- Column sorting (click header or press `s` + column letter)
- Outlier highlighting (cells differing from majority are yellow)
- Toggle between "outliers only" and "full matrix" with `o`

### Body — Tab 4: Tide Branches

Branch coverage overview per repo.

```
  Repo                              Branches
  ──────────────────────────────────────────────────────────────
  openshift/velero                  oadp-1.4, oadp-1.5, oadp-1.6
  openshift/oadp-operator           oadp-1.4, oadp-1.5, oadp-1.6
  migtools/oadp-non-admin           ⚠ No includedBranches (uses all-branch query)
```

---

## Navigation and Keybindings

### Global (all tabs)

| Key | Action |
|-----|--------|
| `1`-`4` | Switch tab |
| `Tab` / `Shift+Tab` | Next/previous tab |
| `r` | Refresh (re-run audit.sh) |
| `q` / `Ctrl+C` | Quit |
| `?` | Help overlay (centered popup, Esc to dismiss) |
| `/` | Open filter prompt (fuzzy search across visible content) |
| `Esc` | Close filter / help / detail view / go back |

### List navigation (tabs 1, 2, 4)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Ctrl+D` | Half-page down |
| `Ctrl+U` | Half-page up |
| `PgDn` / `PgUp` | Full page down/up |
| `Enter` / `Space` | Toggle expand (section header or repo detail) |
| `e` | Expand all sections |
| `c` | Collapse all sections |

### Tab 1 specific

| Key | Action |
|-----|--------|
| `m` | Check merge queue for repo under cursor (opens tab 2 focused) |
| `R` | Refresh single repo under cursor |
| `o` | Open repo's prow config in browser (`gh browse`-style) |

### Tab 2 specific

| Key | Action |
|-----|--------|
| `M` | Check merge queue for all repos |
| `o` | Open PR in browser |
| `p` | Open Prow PR view in browser |

### Tab 3 specific

| Key | Action |
|-----|--------|
| `s` | Cycle sort column |
| `o` | Toggle outliers-only vs full matrix |

### Mouse support

| Action | Effect |
|--------|--------|
| Left click on tab | Switch tab |
| Left click on section header | Toggle collapse |
| Left click on repo row | Select + expand detail |
| Right click on repo | Refresh that repo |
| Scroll wheel | Navigate up/down |
| Click `[r]efresh` in footer | Full refresh |

---

## Component Breakdown

### Go module structure

```
tools/prow-merge-bot-configs/tui/
├── DESIGN.md           # this file
├── go.mod              # module: github.com/oadp-rebasebot/oadp-rebase/tools/prow-audit-tui
├── go.sum
├── main.go             # entry point, flags, subprocess invocation
├── types.go            # JSON contract types (RepoAudit, Finding, PRStatus, etc.)
├── model.go            # top-level Bubbletea model (tabs, global state)
├── tab_audit.go        # Tab 1: config audit list view
├── tab_queue.go        # Tab 2: merge queue view
├── tab_compare.go      # Tab 3: comparison table view
├── tab_tide.go         # Tab 4: tide branch coverage view
├── header.go           # header bar component
├── footer.go           # context-sensitive footer component
├── help.go             # help overlay popup
├── filter.go           # search/filter text input + logic
├── theme.go            # semantic color palette, styles
├── exec.go             # subprocess management (run audit.sh, parse output)
└── util.go             # shared helpers
```

### Dependencies

```go
require (
    github.com/charmbracelet/bubbletea   // TUI framework (Elm architecture)
    github.com/charmbracelet/lipgloss    // Styling + layout composition
    github.com/charmbracelet/bubbles     // Table, viewport, textinput, spinner, help, key
)
```

No other dependencies. The Go binary shells out to `audit.sh` for data collection and `gh` / browser commands for external actions.

### Key types

```go
// types.go

type AuditReport struct {
    Source    string     `json:"source"`
    Timestamp time.Time `json:"timestamp"`
    RateLimit RateLimit `json:"rate_limit"`
    Groups   []RepoGroup `json:"groups"`
    Comparison map[string]map[string]FieldComparison `json:"comparison"`
    Summary  Summary    `json:"summary"`
}

type RateLimit struct {
    Remaining int       `json:"remaining"`
    Limit     int       `json:"limit"`
    ResetAt   time.Time `json:"reset_at"`
}

type RepoGroup struct {
    Name        string      `json:"name"`
    Type        string      `json:"type"`  // upstream-rebase | oadp-owned-openshift | oadp-owned-migtools
    Description string      `json:"description"`
    Repos       []RepoAudit `json:"repos"`
}

type RepoAudit struct {
    Name      string            `json:"name"`       // "openshift/velero"
    HasConfig bool              `json:"has_config"`
    Fields    map[string]string `json:"fields"`     // field name → value
    Plugins   PluginStatus      `json:"plugins"`
    Tide      TideConfig        `json:"tide"`
    Findings  []Finding         `json:"findings"`
}

type Finding struct {
    Severity string `json:"severity"` // ok | warning | issue | info
    Field    string `json:"field"`
    Message  string `json:"message"`
}

type FieldComparison struct {
    Majority string    `json:"majority"`
    Outliers []Outlier `json:"outliers"`
}

type Outlier struct {
    Repo  string `json:"repo"`
    Value string `json:"value"`
}

type MergeQueueReport struct {
    Repo            string `json:"repo"`
    RequiredReviews int    `json:"required_reviews"`
    EnforceAdmins   bool   `json:"enforce_admins"`
    TideURL         string `json:"tide_url"`
    PRs             []PRStatus `json:"prs"`
}

type PRStatus struct {
    Number            int      `json:"number"`
    Title             string   `json:"title"`
    Author            string   `json:"author"`
    Base              string   `json:"base"`
    Head              string   `json:"head"`
    ApprovalCount     int      `json:"approval_count"`
    RequiredReviews   int      `json:"required_reviews"`
    ApprovingReviewers []string `json:"approving_reviewers"`
    LabelBlockers     []string `json:"label_blockers"`
    Checks            CheckStatus `json:"checks"`
    HasBlockers       bool     `json:"has_blockers"`
    ReviewBlocked     bool     `json:"review_blocked"`
    ReviewsShort      int      `json:"reviews_short"`
    PRURL             string   `json:"pr_url"`
    ProwURL           string   `json:"prow_url"`
}

type CheckStatus struct {
    Failing    []string `json:"failing"`
    Errored    []string `json:"errored"`
    Pending    []string `json:"pending"`
    InProgress []string `json:"in_progress"`
    TideState  string   `json:"tide_state"`
}
```

### Bubbletea model structure

```go
// model.go

type model struct {
    // Global state
    width, height int
    activeTab     int           // 0-3
    report        *AuditReport  // current audit data
    mergeQueues   map[string]*MergeQueueReport // repo → queue data (loaded on demand)
    loading       bool
    loadingMsg    string
    err           error

    // Sub-models (each tab is its own Bubbletea model)
    auditTab   auditModel    // Tab 1
    queueTab   queueModel    // Tab 2
    compareTab compareModel  // Tab 3
    tideTab    tideModel     // Tab 4

    // Shared components
    header     headerModel
    footer     footerModel
    help       helpModel     // overlay, toggled with ?
    filter     filterModel   // search input, toggled with /
    spinner    spinner.Model

    // Subprocess
    auditCmd   string        // path to audit.sh
    auditArgs  []string      // extra args (--branch, --local)
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        runAudit(m.auditCmd, m.auditArgs), // async subprocess
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Propagate to all sub-models
    case tea.KeyMsg:
        // Global keys first (tab switch, quit, help, filter)
        // Then delegate to active tab
    case auditResultMsg:
        // Parse JSON, populate m.report, rebuild tab models
    case mergeQueueResultMsg:
        // Populate m.mergeQueues[repo], update queue tab
    case errMsg:
        m.err = msg.err
    }
    return m, nil
}

func (m model) View() string {
    header := m.header.View(m.width, m.report, m.loading)
    tabs := renderTabs(m.activeTab, m.width)
    body := m.activeTabView()
    footer := m.footer.View(m.width, m.activeTab, m.contextKeys())

    // Compose vertically with Lipgloss
    bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer)
    body = lipgloss.NewStyle().Height(bodyHeight).Render(body)

    screen := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)

    // Overlay help popup if active
    if m.help.visible {
        screen = m.help.Overlay(screen, m.width, m.height)
    }

    return screen
}
```

---

## Theme / Styling

Semantic color palette (adapted from k9s + gh-dash patterns):

```go
// theme.go

var (
    // Status colors
    ColorOK      = lipgloss.Color("#4CAF50") // green
    ColorWarn     = lipgloss.Color("#FF9800") // orange/yellow
    ColorIssue    = lipgloss.Color("#F44336") // red
    ColorInfo     = lipgloss.Color("#2196F3") // blue
    ColorDim      = lipgloss.Color("#666666") // gray
    ColorAccent   = lipgloss.Color("#00BCD4") // cyan

    // Structural colors
    ColorHeader   = lipgloss.Color("#FFFFFF")
    ColorBorder   = lipgloss.Color("#444444")
    ColorTabActive = lipgloss.Color("#00BCD4")
    ColorTabInactive = lipgloss.Color("#666666")

    // Styles
    HeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorHeader).
        Background(lipgloss.Color("#1a1a2e")).
        Padding(0, 1)

    TabActiveStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorTabActive).
        Border(lipgloss.NormalBorder(), false, false, true, false).
        BorderForeground(ColorTabActive)

    TabInactiveStyle = lipgloss.NewStyle().
        Foreground(ColorTabInactive)

    FooterStyle = lipgloss.NewStyle().
        Foreground(ColorDim).
        Background(lipgloss.Color("#1a1a2e")).
        Padding(0, 1)

    // Row styles for the audit list
    RowOK    = lipgloss.NewStyle().Foreground(ColorOK)
    RowWarn  = lipgloss.NewStyle().Foreground(ColorWarn)
    RowIssue = lipgloss.NewStyle().Foreground(ColorIssue)
    RowDim   = lipgloss.NewStyle().Foreground(ColorDim)

    // Section header
    SectionStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorAccent).
        Border(lipgloss.NormalBorder(), false, false, true, false).
        BorderForeground(ColorBorder).
        MarginTop(1)
)
```

Unicode indicators:

| Status | Symbol | Color |
|--------|--------|-------|
| OK | `✓` | green |
| Warning | `⚠` | yellow/orange |
| Issue | `✗` | red |
| Info | `ℹ` | blue |
| Pending | `◌` | gray |
| Running | `◉` | yellow |
| Collapsed | `▶` | — |
| Expanded | `▼` | — |

---

## Subprocess Management

```go
// exec.go

// runAudit executes audit.sh --format json and returns an auditResultMsg.
func runAudit(cmd string, args []string) tea.Cmd {
    return func() tea.Msg {
        fullArgs := append(args, "--format", "json", "--skip-queue")
        out, err := exec.Command(cmd, fullArgs...).Output()
        if err != nil {
            return errMsg{err}
        }
        var report AuditReport
        if err := json.Unmarshal(out, &report); err != nil {
            return errMsg{fmt.Errorf("parsing audit JSON: %w", err)}
        }
        return auditResultMsg{report}
    }
}

// runMergeQueue executes audit.sh for a single repo's merge queue.
func runMergeQueue(cmd string, args []string, repo string) tea.Cmd {
    return func() tea.Msg {
        fullArgs := append(args, "--merge-queue", repo, "--format", "json")
        out, err := exec.Command(cmd, fullArgs...).Output()
        if err != nil {
            return errMsg{err}
        }
        var report MergeQueueReport
        if err := json.Unmarshal(out, &report); err != nil {
            return errMsg{fmt.Errorf("parsing merge queue JSON: %w", err)}
        }
        return mergeQueueResultMsg{report}
    }
}
```

### Refresh behavior

- **Full refresh (`r`)**: Re-runs `audit.sh --format json`. A spinner shows in the header. When the subprocess completes, the report is swapped atomically — cursor position, scroll offset, tab selection, and collapsed state are all preserved.
- **Single-repo refresh (`R`)**: Re-runs the full audit (the bash script re-fetches only the changed repo's cache). The TUI diffs the new report against the old one and preserves all UI state.
- **Merge queue check (`m`)**: Runs asynchronously. The queue tab shows a spinner for that repo while loading. Other tabs remain interactive.

---

## CLI Interface

```
Usage: prow-audit-tui [flags]

Flags:
  --audit-script PATH   Path to audit.sh (default: auto-detect from binary location)
  --branch BRANCH       Branch of openshift/release (default: main)
  --local PATH          Use local openshift/release checkout
  --no-mouse            Disable mouse support
  --dump                Run audit and print JSON to stdout (no TUI)
```

The binary auto-detects `audit.sh` relative to its own location (`../audit.sh` or same directory).

---

## Implementation Phases

### Phase 1: JSON output + minimal TUI

**audit.sh changes:**
- Add `--format json` output mode (new code path in `run_audit`, reusing existing check functions)
- Add `--merge-queue <repo>` flag for single-repo queue check with JSON output

**Go TUI:**
- `main.go` + `types.go` + `exec.go`: Parse JSON, run subprocess
- `model.go` + `header.go` + `footer.go`: Basic screen structure
- `tab_audit.go`: Single-tab list view with collapsible sections (feature parity with bash TUI)
- `theme.go`: Color palette
- Navigation: `j/k`, `g/G`, `Enter`, `e/c`, `q`, `?`

**Exit criteria:** Feature parity with the bash `tui_viewer`, but with preserved state on refresh and a header bar.

### Phase 2: Search, filter, and tabs

- `filter.go`: Fuzzy search with `/` key, highlights matches, filters visible rows
- `tab_compare.go`: Interactive comparison table (Bubbles table component)
- `tab_tide.go`: Branch coverage table
- Tab switching with `1`-`4` and `Tab`
- Context-sensitive footer that changes per tab and cursor position
- `help.go`: Centered help overlay with `?`
- Page navigation: `PgUp/PgDn`, `Ctrl+U/D`

**Exit criteria:** All four tabs working, search filters across all tabs.

### Phase 3: Merge queue and polish

- `tab_queue.go`: Full merge queue view with PR details
- Async merge queue loading with per-repo spinners
- `m` on a repo in tab 1 → loads queue data and switches to tab 2 focused on that repo
- `M` loads all repos' merge queues in background
- `o` opens URLs in browser
- Mouse click targets for tabs, sections, refresh button
- Scroll position indicator in footer (`[12/45]`)

### Phase 4: Advanced features (stretch)

- Column sorting in comparison table
- Outliers-only toggle
- Right-click context menu
- Config file for persistent preferences (collapsed sections, default tab)
- `--watch` mode with auto-refresh interval

---

## Comparison: audit.sh vs Go TUI

| Capability | audit.sh (bash) | Go TUI (this design) |
|---|---|---|
| Multi-pane layout | Single scroll | Header + tabs + body + footer |
| Data model | ANSI text file (pager) | Typed Go structs (sort/filter/group) |
| Search/filter | Not possible | Fuzzy filter with `/` |
| State on refresh | Lost (re-enters TUI) | Preserved (atomic data swap) |
| Tables | Free-form text | Bubbles table (aligned, sortable) |
| Rendering | Full-screen clear | Diff-based (Bubbletea, no flicker) |
| Page navigation | `j/k` only | `j/k` + PgUp/PgDn + Ctrl+U/D |
| Help | Squeezed in status bar | `?` centered overlay popup |
| Footer | Static | Context-sensitive per tab/cursor |
| Horizontal scroll | Truncated | Viewport with horizontal scroll |
| Scroll indicator | None | `[12/45]` in footer |
| Async operations | File polling every 500ms | Bubbletea `Cmd` system (event-driven) |
| Input parsing | Manual byte-by-byte `read` | `tea.KeyMsg` (handles all terminals) |
| Testing | Not feasible | Standard Go testing |
| Binary size | 0 (bash) | ~10MB (single static binary) |
| Dependencies | bash, curl, jq, gh | Go binary + audit.sh |

---

## Build and Distribution

```makefile
# In tools/prow-merge-bot-configs/tui/Makefile

BINARY=prow-audit-tui

build:
	go build -o $(BINARY) .

install: build
	cp $(BINARY) ../$(BINARY)

run: build
	./$(BINARY) --audit-script ../audit.sh
```

The Go binary is committed as source, not as a compiled binary. Users build with `go build` or `make build` in the `tui/` directory. The `audit.sh` script remains the primary entry point for non-interactive use.

---

## Open Questions

1. **Should the Go TUI replace the bash interactive mode entirely, or coexist?**
   Recommendation: Coexist. The bash TUI remains the zero-dependency fallback. `audit.sh --format interactive` uses the bash TUI; running `prow-audit-tui` uses the Go TUI. Over time, the bash TUI can be deprecated if the Go version proves reliable.

2. **Should `--format json` live in audit.sh or be a separate Go command?**
   Recommendation: In audit.sh. The bash script already does all the data fetching and checking. Adding JSON serialization to bash is straightforward (`jq` or `printf`-based). This avoids duplicating the audit logic.

3. **Should we vendor Charm dependencies or use Go modules?**
   Recommendation: Go modules (same as `tools/rebase-status/`). The Charm ecosystem is stable and widely used. Vendoring adds ~50MB to the repo for no benefit.
