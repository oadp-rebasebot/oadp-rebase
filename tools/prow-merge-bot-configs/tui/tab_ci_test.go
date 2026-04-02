package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCIGroups() []RepoGroup {
	return []RepoGroup{
		{
			Name: "Test Group",
			Type: "upstream-rebase",
			Repos: []RepoAudit{
				{Name: "openshift/velero"},
				{Name: "openshift/restic"},
			},
		},
	}
}

func testCIInfos() map[string]*CIInfo {
	return map[string]*CIInfo{
		"openshift/velero": {
			Repo:       "openshift/velero",
			GoVersion:  "1.22",
			HasProw:    true,
			HasActions: true,
			ProwJobs: []ProwJob{
				{Name: "e2e-aws", Type: "presubmit", AlwaysRun: true},
			},
			ActionsWflows: []ActionsWorkflow{
				{Filename: "ci.yaml", Name: "CI", Triggers: []string{"push", "pull_request"}},
			},
		},
		"openshift/restic": {
			Repo:      "openshift/restic",
			GoVersion: "1.21",
			HasProw:   true,
		},
	}
}

func TestCIModel_NewBuildRows(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())
	if len(m.rows) == 0 {
		t.Fatal("expected rows")
	}
	if m.rows[0].kind != rowGroupHeader {
		t.Error("expected first row to be group header")
	}
}

func TestCIModel_CollapseExpandGroup(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())
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

func TestCIModel_ExpandRepoJobs(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())

	// Find first repo summary.
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

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expanded[repoName] {
		t.Error("expected repo expanded")
	}
	if len(m.rows) <= rowsBefore {
		t.Error("expected more rows with expanded jobs")
	}
}

func TestCIModel_LeftRightKeys(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())
	m.cursor = 0

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !m.collapsed[0] {
		t.Error("expected collapsed after 'h'")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.collapsed[0] {
		t.Error("expected expanded after 'l'")
	}
}

func TestCIModel_ExpandCollapseAll(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !m.collapsed[0] {
		t.Error("expected collapsed after 'c'")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.collapsed[0] {
		t.Error("expected expanded after 'e'")
	}
}

func TestCIModel_CursorRepo(t *testing.T) {
	m := newCIModel(testCIGroups(), testCIInfos())

	// On header, no repo.
	if r := m.CursorRepo(); r != "" {
		t.Errorf("CursorRepo on header = %q, want empty", r)
	}

	// Move to repo.
	for i, row := range m.rows {
		if row.kind == rowRepoSummary {
			m.cursor = i
			break
		}
	}
	if r := m.CursorRepo(); r == "" {
		t.Error("expected non-empty CursorRepo on repo summary")
	}
}

func TestCIModel_NilCIInfo(t *testing.T) {
	// Should handle nil CI info gracefully.
	m := newCIModel(testCIGroups(), nil)
	if len(m.rows) == 0 {
		t.Fatal("expected rows even with nil ciInfos")
	}
}

func TestCIModel_CIOperatorBranches(t *testing.T) {
	groups := []RepoGroup{
		{
			Name: "Test Group",
			Type: "upstream-rebase",
			Repos: []RepoAudit{
				{Name: "openshift/velero"},
			},
		},
	}
	ciInfos := map[string]*CIInfo{
		"openshift/velero": {
			Repo:          "openshift/velero",
			HasProw:       true,
			HasCIOperator: true,
			CIOperatorConfigCount: 2,
			CIOperatorBranches: []CIOperatorBranch{
				{
					Branch: "release-4.16",
					Tests: []CIOperatorTest{
						{Name: "e2e-aws"},
						{Name: "unit"},
					},
				},
				{
					Branch: "release-4.17",
					Tests: []CIOperatorTest{
						{Name: "e2e-aws"},
						{Name: "e2e-gcp"},
						{Name: "lint"},
					},
				},
			},
		},
	}

	m := newCIModel(groups, ciInfos)

	// Find repo summary and expand it.
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
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expanded["openshift/velero"] {
		t.Fatal("expected repo expanded")
	}

	// Should now have branch header rows.
	branchIdx := -1
	for i, row := range m.rows {
		if row.kind == rowBranchHeader {
			branchIdx = i
			break
		}
	}
	if branchIdx < 0 {
		t.Fatal("no branch header row found after expanding repo")
	}

	// Verify collapsed branch shows count.
	branchRow := m.rows[branchIdx]
	if branchRow.branchKey != "openshift/velero:release-4.16" {
		t.Errorf("branchKey = %q, want %q", branchRow.branchKey, "openshift/velero:release-4.16")
	}
	if !strings.Contains(branchRow.text, "2 tests") {
		t.Errorf("branch text %q should contain '2 tests'", branchRow.text)
	}

	// Expand the branch.
	rowsBefore := len(m.rows)
	m.cursor = branchIdx
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expandedBranches["openshift/velero:release-4.16"] {
		t.Error("expected branch expanded")
	}
	if len(m.rows) <= rowsBefore {
		t.Error("expected more rows with expanded branch tests")
	}

	// Verify test rows appear.
	foundTest := false
	for _, row := range m.rows {
		if row.kind == rowRepoDetail && row.branchKey == "openshift/velero:release-4.16" {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Error("expected test detail rows under expanded branch")
	}

	// Collapse branch with left arrow.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.expandedBranches["openshift/velero:release-4.16"] {
		t.Error("expected branch collapsed after 'h'")
	}
}

