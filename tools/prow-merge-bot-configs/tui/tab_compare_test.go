package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCompareReport() *AuditReport {
	return &AuditReport{
		Comparison: map[string]map[string]FieldComparison{
			"upstream-rebase": {
				"allow_force_pushes": {
					Majority: "true",
					Outliers: []Outlier{{Repo: "openshift/restic", Value: "NOT_SET"}},
				},
				"merge_method": {
					Majority: "NOT_SET",
					Outliers: nil, // all consistent
				},
			},
		},
	}
}

func TestCompareModel_NewBuildRows(t *testing.T) {
	m := newCompareModel(testCompareReport())
	if len(m.rows) == 0 {
		t.Fatal("expected rows after newCompareModel")
	}
}

func TestCompareModel_NilReport(t *testing.T) {
	m := newCompareModel(nil)
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 row for nil report, got %d", len(m.rows))
	}
}

func TestCompareModel_CollapseExpandGroup(t *testing.T) {
	m := newCompareModel(testCompareReport())

	// Find header row.
	headerIdx := -1
	for i, row := range m.rows {
		if row.isHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("no header row found")
	}

	m.cursor = headerIdx
	rowsBefore := len(m.rows)

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.collapsed[m.rows[headerIdx].groupIdx] {
		t.Error("expected group collapsed after enter")
	}
	if len(m.rows) >= rowsBefore {
		t.Error("expected fewer rows after collapse")
	}

	// Find header again and expand.
	for i, row := range m.rows {
		if row.isHeader {
			m.cursor = i
			break
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestCompareModel_OutliersOnlyToggle(t *testing.T) {
	m := newCompareModel(testCompareReport())

	if m.showOutliersOnly {
		t.Error("expected showOutliersOnly=false by default")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !m.showOutliersOnly {
		t.Error("expected showOutliersOnly=true after 'o'")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if m.showOutliersOnly {
		t.Error("expected showOutliersOnly=false after second 'o'")
	}
}

func TestCompareModel_ExpandCollapseAll(t *testing.T) {
	m := newCompareModel(testCompareReport())

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	allCollapsed := true
	for _, v := range m.collapsed {
		if !v {
			allCollapsed = false
		}
	}
	if !allCollapsed {
		t.Error("expected all collapsed after 'c'")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	anyCollapsed := false
	for _, v := range m.collapsed {
		if v {
			anyCollapsed = true
		}
	}
	if anyCollapsed {
		t.Error("expected none collapsed after 'e'")
	}
}

func TestCompareModel_Navigation(t *testing.T) {
	m := newCompareModel(testCompareReport())
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}
