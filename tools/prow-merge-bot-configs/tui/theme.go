package main

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Semantic color palette — colorblind-friendly (blue/orange safe)
//
// Avoids red/green ambiguity. Uses:
//   OK      = blue     (universally distinguishable)
//   Warning = orange   (distinct from blue for all color vision types)
//   Issue   = magenta  (distinct from both blue and orange)
//   Info    = cyan     (lighter, informational)
//   Outlier = yellow   (distinct from all above, used only in compare tab)
//
// All statuses also use distinct unicode icons (✓ ⚠ ✗ ℹ) so color
// is never the sole indicator.
// ---------------------------------------------------------------------------

var (
	ColorOK      = lipgloss.Color("#4A9BDB") // blue (safe for protanopia/deuteranopia)
	ColorWarn    = lipgloss.Color("#E69F00") // orange (universally distinct from blue)
	ColorIssue   = lipgloss.Color("#CC79A7") // magenta/pink (distinct from blue+orange)
	ColorInfo    = lipgloss.Color("#56B4E9") // light blue / cyan
	ColorOutlier = lipgloss.Color("#F0E442") // yellow (distinct for compare outliers)
	ColorDim     = lipgloss.Color("#888888") // gray
	ColorAccent  = lipgloss.Color("#56B4E9") // light blue
	ColorHeader  = lipgloss.Color("#FFFFFF") // white
	ColorBorder  = lipgloss.Color("#444444") // dark gray
	ColorBg      = lipgloss.Color("#1a1a2e") // dark background
	ColorTabActive   = lipgloss.Color("#56B4E9")
	ColorTabInactive = lipgloss.Color("#888888")
)

// ---------------------------------------------------------------------------
// Unicode indicators — paired with colors so status is never color-only
// ---------------------------------------------------------------------------

const (
	IconOK        = "✓"
	IconWarn      = "⚠"
	IconIssue     = "✗"
	IconInfo      = "ℹ"
	IconOutlier   = "◆" // filled diamond for outliers
	IconPending   = "◌"
	IconRunning   = "◉"
	IconCollapsed = "▶"
	IconExpanded  = "▼"
)

// ---------------------------------------------------------------------------
// Reusable Lipgloss styles
// ---------------------------------------------------------------------------

// Header and footer styles.
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHeader).
			Background(ColorBg).
			PaddingLeft(1).
			PaddingRight(1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Background(ColorBg).
			PaddingLeft(1).
			PaddingRight(1)

	FooterKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)
)

// Tab styles.
var (
	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTabActive).
			UnderlineSpaces(true).
			Underline(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorTabInactive)
)

// Section header style.
var SectionHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorAccent)

// Row styles — one per severity / selection state.
var (
	RowOKStyle       = lipgloss.NewStyle().Foreground(ColorOK)
	RowWarnStyle     = lipgloss.NewStyle().Foreground(ColorWarn)
	RowIssueStyle    = lipgloss.NewStyle().Foreground(ColorIssue)
	RowInfoStyle     = lipgloss.NewStyle().Foreground(ColorInfo)
	RowOutlierStyle  = lipgloss.NewStyle().Foreground(ColorOutlier).Bold(true)
	RowDimStyle      = lipgloss.NewStyle().Foreground(ColorDim)
	RowSelectedStyle = lipgloss.NewStyle().Reverse(true)

	// Highlight style for search matches.
	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Background(ColorOutlier).
			Foreground(lipgloss.Color("#000000"))
)

// Help overlay border.
var HelpBorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorAccent).
	Padding(1)

// Status badge styles.
var (
	StatusOKStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorOK)
	StatusWarnStyle  = lipgloss.NewStyle().Bold(true).Foreground(ColorWarn)
	StatusIssueStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorIssue)
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// SeverityStyle returns the appropriate style for a severity level.
func SeverityStyle(s Severity) lipgloss.Style {
	switch s {
	case SeverityOK:
		return RowOKStyle
	case SeverityInfo:
		return RowInfoStyle
	case SeverityWarning:
		return RowWarnStyle
	case SeverityIssue:
		return RowIssueStyle
	default:
		return RowDimStyle
	}
}

// SeverityIcon returns the unicode icon for a severity level.
func SeverityIcon(s Severity) string {
	switch s {
	case SeverityOK:
		return IconOK
	case SeverityInfo:
		return IconInfo
	case SeverityWarning:
		return IconWarn
	case SeverityIssue:
		return IconIssue
	default:
		return IconPending
	}
}
