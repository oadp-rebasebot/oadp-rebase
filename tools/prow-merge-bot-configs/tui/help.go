package main

import (
	"github.com/charmbracelet/lipgloss"
)

type helpModel struct {
	visible bool
}

func (h helpModel) View(width, height int) string {
	if !h.visible {
		return ""
	}

	content := `
  Navigation
    ↑/k     Move up
    ↓/j     Move down
    g       Go to top
    G       Go to bottom
    PgUp    Page up
    PgDn    Page down
    Ctrl+U  Half page up
    Ctrl+D  Half page down

  Actions
    Enter   Toggle expand
    e       Expand all
    c       Collapse all
    r       Refresh
    m       Merge queue
    M       All queues
    a       Show/hide healthy repos

  Search
    /       Open search
    Enter   Apply filter
    Esc     Clear filter

  Tabs
    1-5     Switch tab
    Tab     Next tab

  General
    ?       Toggle help
    q       Quit

  Press ? or Esc to close
`

	box := HelpBorderStyle.Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
