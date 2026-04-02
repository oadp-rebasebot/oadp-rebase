package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testTideGroups() []RepoGroup {
	return []RepoGroup{
		{
			Name: "Test Group",
			Type: "upstream-rebase",
			Repos: []RepoAudit{
				{
					Name: "openshift/velero",
					Tide: TideConfig{IncludedBranches: []string{"main", "oadp-1.4"}},
				},
				{
					Name: "openshift/restic",
					Tide: TideConfig{IncludedBranches: nil}, // no branches
				},
			},
		},
	}
}

func TestTideModel_NewBuildRows(t *testing.T) {
	m := newTideModel(testTideGroups())
	if len(m.rows) == 0 {
		t.Fatal("expected rows")
	}
	// First row should be header.
	if !m.rows[0].isHeader {
		t.Error("expected first row to be header")
	}
}

func TestTideModel_CollapseExpandGroup(t *testing.T) {
	m := newTideModel(testTideGroups())
	m.cursor = 0
	rowsBefore := len(m.rows)

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.collapsed[0] {
		t.Error("expected collapsed after enter")
	}
	if len(m.rows) >= rowsBefore {
		t.Error("expected fewer rows")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.collapsed[0] {
		t.Error("expected expanded after second enter")
	}
}

func TestTideModel_LeftRightKeys(t *testing.T) {
	m := newTideModel(testTideGroups())
	m.cursor = 0

	// Left to collapse.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !m.collapsed[0] {
		t.Error("expected collapsed after 'h'")
	}

	// Right to expand.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.collapsed[0] {
		t.Error("expected expanded after 'l'")
	}
}

func TestTideModel_ExpandCollapseAll(t *testing.T) {
	m := newTideModel(testTideGroups())

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !m.collapsed[0] {
		t.Error("expected collapsed after 'c'")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.collapsed[0] {
		t.Error("expected expanded after 'e'")
	}
}

func TestTideModel_Navigation(t *testing.T) {
	m := newTideModel(testTideGroups())
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestTideModel_NoBranches_Warning(t *testing.T) {
	m := newTideModel(testTideGroups())
	foundWarning := false
	for _, row := range m.rows {
		if row.severity == SeverityWarning {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning row for repo with no includedBranches")
	}
}
