package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testAuditGroups() []RepoGroup {
	return []RepoGroup{
		{
			Name:        "Upstream Rebase Repos",
			Type:        "upstream-rebase",
			Description: "Forks of upstream projects.",
			Repos: []RepoAudit{
				{
					Name: "openshift/velero", HasConfig: true,
					Fields: map[string]string{"allow_force_pushes": "true"},
					Plugins: PluginStatus{HasApproveSection: true, HasLgtmSection: true, HasApprovePlugin: true},
					Findings: []Finding{
						{Severity: SeverityOK, Field: "allow_force_pushes", Message: "ok"},
					},
				},
				{
					Name: "openshift/velero-plugin-for-aws", HasConfig: true,
					Fields: map[string]string{"allow_force_pushes": "NOT_SET"},
					Plugins: PluginStatus{HasApproveSection: true, HasLgtmSection: true, HasApprovePlugin: true},
					Findings: []Finding{
						{Severity: SeverityWarning, Field: "allow_force_pushes", Message: "missing"},
					},
				},
			},
		},
		{
			Name: "OADP-Owned Repos",
			Type: "oadp-owned-openshift",
			Repos: []RepoAudit{
				{
					Name: "openshift/oadp-operator", HasConfig: true,
					Fields: map[string]string{"enforce_admins": "true"},
					Plugins: PluginStatus{HasApproveSection: true, HasLgtmSection: true, HasApprovePlugin: true},
				},
			},
		},
	}
}

func TestAuditModel_NewBuildRows(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)
	if len(m.rows) == 0 {
		t.Fatal("expected rows after newAuditModel")
	}
	// First row should be a group header.
	if m.rows[0].kind != rowGroupHeader {
		t.Errorf("first row kind = %d, want rowGroupHeader (%d)", m.rows[0].kind, rowGroupHeader)
	}
}

func TestAuditModel_CollapseExpandGroup(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)
	initialRows := len(m.rows)

	// Cursor is on first group header. Press enter to collapse.
	m.cursor = 0
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.collapsed[0] {
		t.Error("expected group 0 to be collapsed after enter")
	}
	if len(m.rows) >= initialRows {
		t.Error("expected fewer rows after collapsing group")
	}

	// Press enter again to expand.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.collapsed[0] {
		t.Error("expected group 0 to be expanded after second enter")
	}
	if len(m.rows) != initialRows {
		t.Errorf("expected %d rows after expanding, got %d", initialRows, len(m.rows))
	}
}

func TestAuditModel_CollapseWithLeftKey(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)
	m.cursor = 0

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !m.collapsed[0] {
		t.Error("expected group 0 collapsed after 'h'")
	}
}

func TestAuditModel_ExpandWithRightKey(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)
	m.cursor = 0
	m.collapsed[0] = true
	m.buildRows()

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.collapsed[0] {
		t.Error("expected group 0 expanded after 'l'")
	}
}

func TestAuditModel_ExpandAllCollapseAll(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)

	// Collapse all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	for gi := range m.groups {
		if !m.collapsed[gi] {
			t.Errorf("expected group %d collapsed after 'C'", gi)
		}
	}
	collapsedRows := len(m.rows)

	// Expand all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for gi := range m.groups {
		if m.collapsed[gi] {
			t.Errorf("expected group %d expanded after 'e'", gi)
		}
	}
	if len(m.rows) <= collapsedRows {
		t.Error("expected more rows after expanding all")
	}
}

func TestAuditModel_ExpandRepoDetail(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)

	// Find the first repo summary row.
	repoIdx := -1
	for i, row := range m.rows {
		if row.kind == rowRepoSummary {
			repoIdx = i
			break
		}
	}
	if repoIdx < 0 {
		t.Fatal("no repo summary row found")
	}

	m.cursor = repoIdx
	repoName := m.rows[repoIdx].repoName
	rowsBefore := len(m.rows)

	// Press enter to expand repo details.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expanded[repoName] {
		t.Error("expected repo to be expanded after enter")
	}
	if len(m.rows) <= rowsBefore {
		t.Error("expected more rows after expanding repo detail")
	}

	// Press enter again to collapse.
	// Re-find the cursor position since rows changed.
	for i, row := range m.rows {
		if row.kind == rowRepoSummary && row.repoName == repoName {
			m.cursor = i
			break
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.expanded[repoName] {
		t.Error("expected repo to be collapsed after second enter")
	}
}

func TestAuditModel_CursorNavigation(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)

	// Move down with j.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("cursor = %d after 'j', want 1", m.cursor)
	}

	// Move up with k.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor = %d after 'k', want 0", m.cursor)
	}

	// Go to end with G.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.cursor != len(m.rows)-1 {
		t.Errorf("cursor = %d after 'G', want %d", m.cursor, len(m.rows)-1)
	}

	// Go to start with g.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m.cursor != 0 {
		t.Errorf("cursor = %d after 'g', want 0", m.cursor)
	}
}

func TestAuditModel_CursorRepo(t *testing.T) {
	m := newAuditModel(testAuditGroups(), nil, nil)

	// On group header, CursorRepo should be empty.
	m.cursor = 0
	if r := m.CursorRepo(); r != "" {
		t.Errorf("CursorRepo on group header = %q, want empty", r)
	}

	// Move to a repo summary row.
	for i, row := range m.rows {
		if row.kind == rowRepoSummary {
			m.cursor = i
			break
		}
	}
	if r := m.CursorRepo(); r == "" {
		t.Error("CursorRepo on repo summary should return repo name")
	}
}

func TestAuditModel_ViewEmpty(t *testing.T) {
	m := newAuditModel(nil, nil, nil)
	view := m.View(80, 20, "")
	if view == "" {
		t.Error("expected non-empty view for empty audit model")
	}
}
