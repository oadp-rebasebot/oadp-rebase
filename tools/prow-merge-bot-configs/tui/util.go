package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// openBrowser opens a URL in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("open", url)
	}
	_ = cmd.Start()
}

// --- Shared scroll/cursor helpers used by all tab models ---

// clampScroll clamps cursor and scrollOff to valid ranges given a row count.
// visibleHeight is hardcoded to 20 as a fallback when actual height isn't passed.
func clampScroll(cursor, scrollOff, rowCount int) (int, int) {
	if rowCount == 0 {
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= rowCount {
		cursor = rowCount - 1
	}
	visibleHeight := 20
	if cursor < scrollOff {
		scrollOff = cursor
	}
	if cursor >= scrollOff+visibleHeight {
		scrollOff = cursor - visibleHeight + 1
	}
	if scrollOff < 0 {
		scrollOff = 0
	}
	return cursor, scrollOff
}

// handleNavKeys processes common navigation keys (j/k/g/G/pgup/pgdn/ctrl+u/d).
// Returns the updated cursor value and true if the key was handled.
func handleNavKeys(msg tea.KeyMsg, cursor, rowCount int) (int, bool) {
	switch msg.String() {
	case "j", "down":
		return cursor + 1, true
	case "k", "up":
		return cursor - 1, true
	case "g", "home":
		return 0, true
	case "G", "end":
		if rowCount > 0 {
			return rowCount - 1, true
		}
		return cursor, true
	case "pgdown":
		return cursor + 20, true
	case "pgup":
		return cursor - 20, true
	case "ctrl+d":
		return cursor + 10, true
	case "ctrl+u":
		return cursor - 10, true
	}
	return cursor, false
}

// padLine pads a line with spaces to fill the given width.
func padLine(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth < width {
		return line + strings.Repeat(" ", width-lineWidth)
	}
	return line
}

// padLineWithURL pads a line and appends a 🌐 indicator if url is non-empty.
func padLineWithURL(line string, url string, width int) string {
	if url != "" {
		line += " 🌐"
	}
	return padLine(line, width)
}

// appendScrollIndicator adds a "[cursor/total]" indicator to the last line of content.
func appendScrollIndicator(content string, cursor, rowCount, width int) string {
	if rowCount == 0 {
		return content
	}
	indicator := fmt.Sprintf(" [%d/%d]", cursor+1, rowCount)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		lastIdx := len(lines) - 1
		lastLine := lines[lastIdx]
		lastWidth := lipgloss.Width(lastLine)
		indWidth := lipgloss.Width(indicator)
		if lastWidth+indWidth < width {
			gap := width - lastWidth - indWidth
			if gap < 0 {
				gap = 0
			}
			lines[lastIdx] = lastLine + strings.Repeat(" ", gap) + RowDimStyle.Render(indicator)
		}
		return strings.Join(lines, "\n")
	}
	return content
}



// toString converts an interface value to string (replaces repeated fmt.Sprintf("%v", val)).
func toString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// matchesFilter returns true if text contains query (case-insensitive).
func matchesFilter(text, query string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

// highlightMatch wraps case-insensitive matches of query in the line with HighlightStyle.
// Returns the original line if query is empty.
func highlightMatch(line, query string) string {
	if query == "" {
		return line
	}
	lower := strings.ToLower(line)
	lowerQ := strings.ToLower(query)
	qLen := len(lowerQ)

	var b strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], lowerQ)
		if idx < 0 {
			b.WriteString(line[pos:])
			break
		}
		b.WriteString(line[pos : pos+idx])
		b.WriteString(HighlightStyle.Render(line[pos+idx : pos+idx+qLen]))
		pos += idx + qLen
	}
	return b.String()
}
