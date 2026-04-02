package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// tabDescriptions explains what each tab shows.
var tabDescriptions = [5]string{
	"click [m] check queue · double-click open config · 🌐 = has link",
	"click [m] check repo · click [M] check all · double-click open link · 🌐 = has link",
	"◆ = outlier (differs from majority)",
	"click/double-click 🌐 to open prowconfig · 🌐 = has link",
	"Prow jobs + GitHub Actions · Go version · 🌐 = has link",
}

func renderFooter(width int, activeTab int, _ bool, hasFilter bool, cursorRepo string, watchActive bool) string {
	var hints []string

	// Always present.
	hints = append(hints, FooterKeyStyle.Render("↑↓")+" navigate")

	// Tab-specific hints.
	switch activeTab {
	case 0: // config audit
		hints = append(hints, FooterKeyStyle.Render("Enter")+" toggle")
		hints = append(hints, FooterKeyStyle.Render("←/→")+" collapse/expand")
		hints = append(hints, FooterKeyStyle.Render("e/c")+" all")
		hints = append(hints, FooterKeyStyle.Render("r")+" refresh")
		if cursorRepo != "" {
			hints = append(hints, FooterKeyStyle.Render("m")+" queue")
		}
	case 1: // merge queue
		hints = append(hints, FooterKeyStyle.Render("Enter")+" toggle")
		hints = append(hints, FooterKeyStyle.Render("←")+" collapse")
		hints = append(hints, FooterKeyStyle.Render("→/o")+" open in browser")
		hints = append(hints, FooterKeyStyle.Render("e/c")+" all")
		hints = append(hints, FooterKeyStyle.Render("m")+" check repo")
		hints = append(hints, FooterKeyStyle.Render("s")+" sort")
		hints = append(hints, FooterKeyStyle.Render("a")+" show/hide healthy")
		hints = append(hints, FooterKeyStyle.Render("M")+" check all")
	case 2: // field compare
		hints = append(hints, FooterKeyStyle.Render("Enter")+" toggle")
		hints = append(hints, FooterKeyStyle.Render("e/c")+" all")
		hints = append(hints, FooterKeyStyle.Render("o")+" outliers only")
	case 3: // tide branches
		hints = append(hints, FooterKeyStyle.Render("Enter")+" toggle")
		hints = append(hints, FooterKeyStyle.Render("e/c")+" all")
		hints = append(hints, FooterKeyStyle.Render("→/o")+" open config")
	case 4: // CI
		hints = append(hints, FooterKeyStyle.Render("Enter")+" toggle")
		hints = append(hints, FooterKeyStyle.Render("←/→")+" collapse/expand")
		hints = append(hints, FooterKeyStyle.Render("e/c")+" all")
		hints = append(hints, FooterKeyStyle.Render("o")+" open link")
	}

	hints = append(hints, FooterKeyStyle.Render("/")+" search")
	if hasFilter {
		hints = append(hints, FooterKeyStyle.Render("Esc")+" clear filter")
	}
	if watchActive {
		hints = append(hints, FooterKeyStyle.Render("p")+" pause/resume")
	}

	hints = append(hints, FooterKeyStyle.Render("q")+" quit")
	hints = append(hints, FooterKeyStyle.Render("?")+" help")

	keysLine := strings.Join(hints, " │ ")

	// Tab description, right-aligned on the same line.
	desc := tabDescriptions[activeTab]
	descRendered := RowDimStyle.Render(desc)

	keysWidth := lipgloss.Width(keysLine)
	descWidth := lipgloss.Width(descRendered)

	if keysWidth+descWidth+3 <= width {
		// Fits on one line — right-align the description.
		gap := width - keysWidth - descWidth - 2 // -2 for footer padding
		if gap < 2 {
			gap = 2
		}
		content := keysLine + strings.Repeat(" ", gap) + descRendered
		return FooterStyle.Width(width).Render(content)
	}

	// Too wide — truncate keys progressively, then try again.
	for lipgloss.Width(keysLine) > width-5 && len(hints) > 3 {
		hints = hints[:len(hints)-1]
		keysLine = strings.Join(hints, " │ ") + " …"
	}

	return FooterStyle.Width(width).Render(keysLine)
}

// renderSearchFooter renders the footer when search mode is active.
func renderSearchFooter(width int, input textinput.Model) string {
	prompt := FooterKeyStyle.Render("/") + " " + input.View()
	hint := RowDimStyle.Render("  Enter confirm │ Esc cancel")
	content := prompt + hint
	return FooterStyle.Width(width).Render(content)
}
