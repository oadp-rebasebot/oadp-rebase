package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleNavKeys(t *testing.T) {
	tests := []struct {
		key       string
		cursor    int
		rowCount  int
		wantCur   int
		wantHandl bool
	}{
		{"j", 0, 10, 1, true},
		{"down", 0, 10, 1, true},
		{"k", 5, 10, 4, true},
		{"up", 5, 10, 4, true},
		{"g", 5, 10, 0, true},
		{"home", 5, 10, 0, true},
		{"G", 5, 10, 9, true},
		{"end", 5, 10, 9, true},
		{"G", 0, 0, 0, true}, // empty list
		{"pgdown", 0, 100, 20, true},
		{"pgup", 30, 100, 10, true},
		{"ctrl+d", 0, 100, 10, true},
		{"ctrl+u", 20, 100, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			// Special keys need proper type.
			switch tt.key {
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			case "up":
				msg = tea.KeyMsg{Type: tea.KeyUp}
			case "home":
				msg = tea.KeyMsg{Type: tea.KeyHome}
			case "end":
				msg = tea.KeyMsg{Type: tea.KeyEnd}
			case "pgdown":
				msg = tea.KeyMsg{Type: tea.KeyPgDown}
			case "pgup":
				msg = tea.KeyMsg{Type: tea.KeyPgUp}
			case "ctrl+d":
				msg = tea.KeyMsg{Type: tea.KeyCtrlD}
			case "ctrl+u":
				msg = tea.KeyMsg{Type: tea.KeyCtrlU}
			}

			got, handled := handleNavKeys(msg, tt.cursor, tt.rowCount)
			if handled != tt.wantHandl {
				t.Errorf("handled = %v, want %v", handled, tt.wantHandl)
			}
			if got != tt.wantCur {
				t.Errorf("cursor = %d, want %d", got, tt.wantCur)
			}
		})
	}
}

func TestHandleNavKeys_Unhandled(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	cursor, handled := handleNavKeys(msg, 5, 10)
	if handled {
		t.Error("expected unhandled for 'x' key")
	}
	if cursor != 5 {
		t.Errorf("cursor should remain 5, got %d", cursor)
	}
}

func TestClampScroll(t *testing.T) {
	tests := []struct {
		name               string
		cursor, scroll, rc int
		wantCur, wantScr   int
	}{
		{"empty", 0, 0, 0, 0, 0},
		{"negative cursor", -1, 0, 10, 0, 0},
		{"cursor beyond end", 15, 0, 10, 9, 0},
		{"normal", 5, 0, 10, 5, 0},
		{"scroll follows cursor down", 25, 0, 30, 25, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, s := clampScroll(tt.cursor, tt.scroll, tt.rc)
			if c != tt.wantCur {
				t.Errorf("cursor = %d, want %d", c, tt.wantCur)
			}
			if s != tt.wantScr {
				t.Errorf("scroll = %d, want %d", s, tt.wantScr)
			}
		})
	}
}
