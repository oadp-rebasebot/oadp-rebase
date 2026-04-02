package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQueueDoneMsg_UpdatesRateLimit(t *testing.T) {
	original := fetchRateLimitFn
	defer func() { fetchRateLimitFn = original }()

	newRL := &RateLimit{Remaining: 4900, Limit: 5000, ResetAt: time.Now().UTC()}
	fetchRateLimitFn = func() *RateLimit { return newRL }

	m := model{
		report: &AuditReport{
			RateLimit: &RateLimit{Remaining: 5000, Limit: 5000, ResetAt: time.Now().UTC()},
		},
		queueTab: newQueueModel(),
	}

	msg := queueDoneMsg{repo: "openshift/velero", report: &MergeQueueReport{}}
	newModel, _ := m.Update(msg)
	updated := newModel.(model)

	if updated.report.RateLimit.Remaining != 4900 {
		t.Errorf("RateLimit.Remaining = %d, want 4900", updated.report.RateLimit.Remaining)
	}
}

func TestQueueDoneMsg_KeepsOldOnFetchFailure(t *testing.T) {
	original := fetchRateLimitFn
	defer func() { fetchRateLimitFn = original }()

	// Mock returns nil (simulating fetch failure).
	fetchRateLimitFn = func() *RateLimit { return nil }

	oldRL := &RateLimit{Remaining: 4500, Limit: 5000, ResetAt: time.Now().UTC()}
	m := model{
		report: &AuditReport{
			RateLimit: oldRL,
		},
		queueTab: newQueueModel(),
	}

	msg := queueDoneMsg{repo: "openshift/velero", report: &MergeQueueReport{}}
	newModel, _ := m.Update(msg)
	updated := newModel.(model)

	// Rate limit should remain unchanged when fetch fails.
	if updated.report.RateLimit != oldRL {
		t.Error("RateLimit should be preserved when fetchRateLimitFn returns nil")
	}
	if updated.report.RateLimit.Remaining != 4500 {
		t.Errorf("RateLimit.Remaining = %d, want 4500 (preserved)", updated.report.RateLimit.Remaining)
	}
}

func TestQueueDoneMsg_NilReport(t *testing.T) {
	original := fetchRateLimitFn
	defer func() { fetchRateLimitFn = original }()

	fetchRateLimitFn = func() *RateLimit {
		return &RateLimit{Remaining: 4900, Limit: 5000, ResetAt: time.Now().UTC()}
	}

	m := model{
		report:   nil, // report is nil
		queueTab: newQueueModel(),
	}

	msg := queueDoneMsg{repo: "openshift/velero", report: &MergeQueueReport{}}
	// Should not panic.
	newModel, _ := m.Update(msg)
	updated := newModel.(model)

	if updated.report != nil {
		t.Error("report should remain nil")
	}
}

func TestAuditDoneMsg_SetsRateLimit(t *testing.T) {
	rl := &RateLimit{Remaining: 4800, Limit: 5000, ResetAt: time.Now().UTC()}
	report := &AuditReport{
		RateLimit: rl,
		Groups: []RepoGroup{
			{
				Name: "Test",
				Type: "upstream-rebase",
				Repos: []RepoAudit{
					{Name: "openshift/velero", HasConfig: true, Fields: map[string]string{}},
				},
			},
		},
	}

	m := model{
		queueTab: newQueueModel(),
	}

	msg := auditDoneMsg{report: report, configs: map[string]*RepoConfig{}}
	newModel, _ := m.Update(msg)
	updated := newModel.(model)

	if updated.report.RateLimit == nil {
		t.Fatal("expected RateLimit to be set after auditDoneMsg")
	}
	if updated.report.RateLimit.Remaining != 4800 {
		t.Errorf("RateLimit.Remaining = %d, want 4800", updated.report.RateLimit.Remaining)
	}
}

func TestModel_CopyFlashMsg(t *testing.T) {
	m := model{
		flash:    "Copied!",
		queueTab: newQueueModel(),
	}

	newModel, _ := m.Update(copyFlashMsg{})
	updated := newModel.(model)

	if updated.flash != "" {
		t.Errorf("flash should be cleared after copyFlashMsg, got %q", updated.flash)
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := model{
		queueTab: newQueueModel(),
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := model{
		queueTab: newQueueModel(),
	}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated := newModel.(model)

	if !updated.help.visible {
		t.Error("expected help to be visible after '?' press")
	}

	// Press '?' again to close.
	newModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated = newModel.(model)

	if updated.help.visible {
		t.Error("expected help to be hidden after second '?' press")
	}
}

func TestModel_TabSwitching(t *testing.T) {
	m := model{
		queueTab: newQueueModel(),
	}

	tests := []struct {
		key  rune
		want int
	}{
		{'2', 1},
		{'3', 2},
		{'4', 3},
		{'1', 0},
	}

	for _, tt := range tests {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		updated := newModel.(model)
		if updated.activeTab != tt.want {
			t.Errorf("key '%c': activeTab = %d, want %d", tt.key, updated.activeTab, tt.want)
		}
		m = updated
	}
}
