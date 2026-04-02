# prow-audit-tui

Interactive terminal dashboard for auditing Prow merge bot configurations across OADP ecosystem repositories.

Fetches `_pluginconfig.yaml` and `_prowconfig.yaml` from [openshift/release](https://github.com/openshift/release) and checks them against expected patterns for each repo type.

## Quick Start

Run directly without cloning (requires Go 1.22+):

```bash
go run github.com/oadp-rebasebot/oadp-rebase/tools/prow-merge-bot-configs/tui@latest
```

## Install

```bash
# Install to $GOPATH/bin
go install github.com/oadp-rebasebot/oadp-rebase/tools/prow-merge-bot-configs/tui@latest

# Or build from source
cd tools/prow-merge-bot-configs/tui
go build -o prow-audit-tui .
```

Requires Go 1.22+.

## Usage

```bash
# Interactive TUI (default when terminal detected)
./prow-audit-tui

# Text output with ANSI colors
./prow-audit-tui --format text

# Markdown output
./prow-audit-tui --format markdown

# JSON output
./prow-audit-tui --format json

# Use local openshift/release checkout (no GitHub API calls)
./prow-audit-tui --local /path/to/openshift/release

# Specific branch
./prow-audit-tui --branch release-4.18
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--branch` | `main` | Branch of openshift/release to audit |
| `--format` | auto | `text`, `markdown`, `json`, or `interactive` (auto-detects) |
| `--local` | — | Path to local openshift/release checkout |
| `--no-mouse` | false | Disable mouse support |
| `--skip-queue` | false | Skip merge queue checks |

### Prerequisites

- **Go 1.22+** to build
- **`gh` CLI** ([install](https://cli.github.com/)) — required for merge queue checks, optional for config audit
- **`GITHUB_TOKEN`** or `gh auth login` — for higher API rate limits (5000/hr vs 60/hr anonymous)

Without `gh` CLI, the config audit still works (fetches via raw.githubusercontent.com). Merge queue features show a warning.

## Tabs

### [1] Config Audit

Audits prow plugin and branch-protection configs per repo.

**Repo groups:**
- **Upstream Rebase** (7 repos) — expects `allow_force_pushes=true` for rebasebot
- **OADP-Owned openshift/** (4 repos) — expects `enforce_admins`, `review_count=2`, `dismiss_stale_reviews`
- **OADP-Owned migtools/** (12 repos) — same expectations as openshift/

**Checks performed:**
- Plugin config: approve/lgtm sections, approve plugin presence, require_self_approval
- Branch protection: enforce_admins, review count, dismiss_stale_reviews, allow_force_pushes (supports all 4 Prow inheritance levels: global/org/repo/branch)
- Tide: merge_method, keep-main-query-separate, required labels (approved/lgtm), blocker labels (do-not-merge/*, needs-rebase, etc.)

### [2] Merge Queue

Shows PRs with `approved+lgtm` labels that haven't merged yet. Identifies what's blocking each PR.

**Key feature: tideErrLoopBlocker detection** — flags PRs that meet label requirements but fail GitHub branch protection review counts when `enforce_admins=true`. These cause Tide to enter an error loop ([prow#134](https://github.com/kubernetes-sigs/prow/issues/134)).

Sort modes (press `s`): name, PR count, blocked count.

### [3] Field Compare

Cross-repo comparison showing which repos differ from the majority value for each config field. Outliers highlighted with `◆` icon in yellow.

### [4] Tide Branches

Shows `includedBranches` from Tide queries for each repo. Warns on repos without explicit branch inclusion.

## Keyboard

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓` | Navigate up/down |
| `g`/`G` | Jump to top/bottom |
| `PgUp`/`PgDn` | Page up/down |
| `Ctrl+U`/`Ctrl+D` | Half-page up/down |
| `Enter`/`Space` | Toggle expand/collapse |
| `←`/`→` | Collapse/expand (→ opens browser if expanded) |
| `e`/`c` | Expand/collapse all |
| `o` | Open URL in browser |
| `m` | Check merge queue for repo under cursor |
| `M` | Check all merge queues |
| `s` | Cycle sort mode (queue tab) |
| `r` | Refresh (re-fetch all configs) |
| `1`-`4` | Switch tab |
| `Tab`/`Shift+Tab` | Next/previous tab |
| `Shift+←`/`Shift+→` | Previous/next tab |
| `?` | Help overlay |
| `q` | Quit |

## Mouse

| Action | Effect |
|--------|--------|
| Click tab bar | Switch tab |
| Click `[m]` button | Check merge queue for that repo |
| Click `[M]` button | Check all merge queues |
| Click section header | Toggle collapse |
| Click repo/PR row | Toggle expand/collapse |
| Click `🌐` globe | Open link in browser |
| Double-click repo (audit) | Open config on GitHub |
| Double-click row (queue) | Open link in browser |
| Scroll wheel | Navigate up/down |

## Colorblind Accessibility

Uses a colorblind-friendly palette (safe for protanopia/deuteranopia):

| Status | Color | Icon |
|--------|-------|------|
| OK | Blue `#4A9BDB` | ✓ |
| Warning | Orange `#E69F00` | ⚠ |
| Issue | Magenta `#CC79A7` | ✗ |
| Info | Cyan `#56B4E9` | ℹ |
| Outlier | Yellow `#F0E442` | ◆ |

All statuses use **both** color **and** distinct icons — color is never the sole indicator.

## Architecture

```
prow-audit-tui (single Go binary, ~10MB)
├── config.go    — Fetch YAML configs (HTTP or local), parse, extract fields
├── audit.go     — Run audit checks per repo type, cross-repo comparison
├── queue.go     — Merge queue via `gh pr list`, check normalization
├── model.go     — Bubbletea TUI model, tab switching, mouse handling
├── tab_audit.go — Config audit view (collapsible groups + repos)
├── tab_queue.go — Merge queue view (collapsible repos + PRs)
├── tab_compare.go — Field comparison view
├── tab_tide.go  — Tide branch coverage view
├── theme.go     — Colorblind-safe palette, styles, icons
├── header.go    — Header bar (source, timestamp, rate limit)
├── footer.go    — Context-sensitive keybinding hints
├── help.go      — Help overlay
├── render_text.go — Text/markdown/JSON output (non-interactive)
├── types.go     — Data types, repo group definitions
├── util.go      — Shared helpers (scroll, pad, browser open)
└── main.go      — Entry point, flags, token detection
```

Dependencies: [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles) + [yaml.v3](https://gopkg.in/yaml.v3).
