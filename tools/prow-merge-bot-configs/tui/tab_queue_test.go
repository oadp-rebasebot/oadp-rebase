package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQueueModel_TideErrLoopBlocker_Detection(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	report := &MergeQueueReport{
		Repo:            "openshift/oadp-operator",
		RequiredReviews: 2,
		EnforceAdmins:   true,
		TideURL:         "https://prow.ci.openshift.org/tide",
		PRs: []PRStatus{
			{
				Number:        1,
				Title:         "Blocker PR",
				Author:        "alice",
				Base:          "main",
				ApprovalCount: 1,
				HasBlockers:   true,
				ReviewBlocked: true,
				ReviewsShort:  1,
				LabelBlockers: nil, // no label blockers -> tideErrLoopBlocker
				PRURL:         "https://github.com/openshift/oadp-operator/pull/1",
			},
			{
				Number:        2,
				Title:         "Ready PR",
				Author:        "bob",
				Base:          "main",
				ApprovalCount: 2,
				HasBlockers:   false,
				ReviewBlocked: false,
				PRURL:         "https://github.com/openshift/oadp-operator/pull/2",
			},
		},
	}

	m.SetQueue("openshift/oadp-operator", report)

	// Check that rows contain tideErrLoopBlocker indicators.
	foundBlockerRow := false
	foundReadyWarning := false
	for _, row := range m.rows {
		if strings.Contains(row.text, "BLOCKS QUEUE") && strings.Contains(row.text, "tideErrLoopBlocker") {
			foundBlockerRow = true
			if row.severity != SeverityIssue {
				t.Errorf("tideErrLoopBlocker row severity = %v, want SeverityIssue", row.severity)
			}
		}
		if strings.Contains(row.text, "tideErrLoopBlocker in repo") {
			foundReadyWarning = true
		}
	}
	if !foundBlockerRow {
		t.Error("expected row with 'BLOCKS QUEUE' and 'tideErrLoopBlocker' text")
	}
	if !foundReadyWarning {
		t.Error("expected warning about tideErrLoopBlocker in repo for ready PR")
	}
}

func TestQueueModel_TideErrLoopBlocker_NotTriggeredWithoutEnforceAdmins(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	report := &MergeQueueReport{
		Repo:            "openshift/oadp-operator",
		RequiredReviews: 2,
		EnforceAdmins:   false, // no enforce_admins -> no tideErrLoopBlocker
		PRs: []PRStatus{
			{
				Number:        1,
				Title:         "PR with low reviews",
				Author:        "alice",
				Base:          "main",
				ApprovalCount: 1,
				HasBlockers:   true,
				ReviewBlocked: true,
				ReviewsShort:  1,
				LabelBlockers: nil,
				PRURL:         "https://github.com/openshift/oadp-operator/pull/1",
			},
		},
	}

	m.SetQueue("openshift/oadp-operator", report)

	for _, row := range m.rows {
		if strings.Contains(row.text, "BLOCKS QUEUE") {
			t.Error("should NOT show 'BLOCKS QUEUE' when enforce_admins=false")
		}
	}
}

func TestQueueModel_TideErrLoopBlocker_NotTriggeredWithLabelBlockers(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	report := &MergeQueueReport{
		Repo:            "openshift/oadp-operator",
		RequiredReviews: 2,
		EnforceAdmins:   true,
		PRs: []PRStatus{
			{
				Number:        1,
				Title:         "Held PR",
				Author:        "alice",
				Base:          "main",
				ApprovalCount: 1,
				HasBlockers:   true,
				ReviewBlocked: true,
				ReviewsShort:  1,
				LabelBlockers: []string{"do-not-merge/hold"}, // has label blocker
				PRURL:         "https://github.com/openshift/oadp-operator/pull/1",
			},
		},
	}

	m.SetQueue("openshift/oadp-operator", report)

	for _, row := range m.rows {
		if strings.Contains(row.text, "BLOCKS QUEUE") {
			t.Error("should NOT show 'BLOCKS QUEUE' when label blockers exist")
		}
	}
}

func TestQueueModel_SetQueue(t *testing.T) {
	m := newQueueModel()

	report := &MergeQueueReport{
		Repo: "openshift/velero",
		PRs:  []PRStatus{},
	}
	m.SetQueue("openshift/velero", report)

	if _, ok := m.queues["openshift/velero"]; !ok {
		t.Error("expected queue to be stored")
	}
	if len(m.repoOrder) != 1 || m.repoOrder[0] != "openshift/velero" {
		t.Errorf("repoOrder = %v, want [openshift/velero]", m.repoOrder)
	}
}

func TestQueueModel_SortModes(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true
	m.hideHealthy = false

	m.SetQueue("b-repo", &MergeQueueReport{Repo: "b-repo", PRs: []PRStatus{{Number: 1, HasBlockers: true, PRURL: "u"}}})
	m.SetQueue("a-repo", &MergeQueueReport{Repo: "a-repo", PRs: []PRStatus{{Number: 1, PRURL: "u"}, {Number: 2, PRURL: "u"}}})

	// Default sort: name.
	repos := m.sortedRepos()
	if repos[0] != "a-repo" {
		t.Errorf("name sort: first = %q, want a-repo", repos[0])
	}

	// Cycle to PR count sort.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.sortMode != queueSortPRCount {
		t.Errorf("sortMode = %d, want queueSortPRCount", m.sortMode)
	}

	// Cycle to blocked sort.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.sortMode != queueSortBlocked {
		t.Errorf("sortMode = %d, want queueSortBlocked", m.sortMode)
	}

	repos = m.sortedRepos()
	if repos[0] != "b-repo" {
		t.Errorf("blocked sort: first = %q, want b-repo (has blocker)", repos[0])
	}
}

func TestQueueModel_HideHealthyToggle(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	if !m.hideHealthy {
		t.Error("expected hideHealthy=true by default")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.hideHealthy {
		t.Error("expected hideHealthy=false after 'a' toggle")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.hideHealthy {
		t.Error("expected hideHealthy=true after second 'a' toggle")
	}
}

func TestQueueModel_CollapseExpandRepo(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	m.SetQueue("openshift/velero", &MergeQueueReport{
		Repo: "openshift/velero",
		PRs:  []PRStatus{{Number: 1, Title: "Test PR", PRURL: "u", HasBlockers: true, LabelBlockers: []string{"hold"}}},
	})

	// Find repo header row.
	headerIdx := -1
	for i, row := range m.rows {
		if row.kind == rowGroupHeader && row.repoName == "openshift/velero" {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("no header row for openshift/velero")
	}

	m.cursor = headerIdx
	rowsBefore := len(m.rows)

	// Collapse with enter.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.collapsed["openshift/velero"] {
		t.Error("expected repo collapsed after enter")
	}
	if len(m.rows) >= rowsBefore {
		t.Error("expected fewer rows after collapse")
	}

	// Expand with enter.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.collapsed["openshift/velero"] {
		t.Error("expected repo expanded after second enter")
	}
}

func TestQueueModel_ExpandCollapseAll(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	m.SetQueue("openshift/velero", &MergeQueueReport{
		Repo: "openshift/velero",
		PRs:  []PRStatus{{Number: 1, Title: "PR", PRURL: "u", HasBlockers: true}},
	})

	// Collapse all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !m.collapsed["openshift/velero"] {
		t.Error("expected collapsed after 'c'")
	}

	// Expand all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.collapsed["openshift/velero"] {
		t.Error("expected expanded after 'e'")
	}
}

func TestQueueModel_RepoIsHealthy(t *testing.T) {
	m := newQueueModel()

	// No data yet -> not healthy.
	if m.repoIsHealthy("openshift/velero") {
		t.Error("expected not healthy when no data")
	}

	// All PRs ready -> healthy.
	m.SetQueue("openshift/velero", &MergeQueueReport{
		Repo: "openshift/velero",
		PRs:  []PRStatus{{Number: 1, HasBlockers: false}},
	})
	if !m.repoIsHealthy("openshift/velero") {
		t.Error("expected healthy when no blockers")
	}

	// Some blockers -> not healthy.
	m.SetQueue("openshift/velero", &MergeQueueReport{
		Repo: "openshift/velero",
		PRs:  []PRStatus{{Number: 1, HasBlockers: true}},
	})
	if m.repoIsHealthy("openshift/velero") {
		t.Error("expected not healthy when blockers exist")
	}
}

func TestQueueModel_HeaderSeverity(t *testing.T) {
	m := newQueueModel()
	m.ghAvailable = true

	// tideErrLoopBlocker PR -> header should be SeverityIssue.
	m.SetQueue("openshift/oadp-operator", &MergeQueueReport{
		Repo:            "openshift/oadp-operator",
		RequiredReviews: 2,
		EnforceAdmins:   true,
		PRs: []PRStatus{
			{
				Number: 1, HasBlockers: true, ReviewBlocked: true,
				LabelBlockers: nil, PRURL: "u",
			},
		},
	})

	for _, row := range m.rows {
		if row.kind == rowGroupHeader && row.repoName == "openshift/oadp-operator" {
			if row.severity != SeverityIssue {
				t.Errorf("header severity = %v, want SeverityIssue for tideErrLoopBlocker", row.severity)
			}
			if !strings.Contains(row.text, "tideErrLoopBlocker") {
				t.Error("expected header text to contain 'tideErrLoopBlocker'")
			}
			break
		}
	}
}
