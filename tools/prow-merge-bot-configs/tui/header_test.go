package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHeader_NilRateLimit(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: nil,
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if strings.Contains(out, "API:") {
		t.Error("expected no 'API:' when RateLimit is nil")
	}
}

func TestRenderHeader_HighRemaining(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 4500, Limit: 5000, ResetAt: time.Now()},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "4500/5000") {
		t.Error("expected '4500/5000' in header")
	}
	if strings.Contains(out, "resets") {
		t.Error("should not show 'resets' when remaining > 100")
	}
}

func TestRenderHeader_LowRemaining(t *testing.T) {
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 50, Limit: 5000, ResetAt: resetAt},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "50/5000") {
		t.Error("expected '50/5000' in header")
	}
	if !strings.Contains(out, "resets") {
		t.Error("expected 'resets' when remaining < 100")
	}
}

func TestRenderHeader_ZeroRemaining(t *testing.T) {
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 0, Limit: 5000, ResetAt: resetAt},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "exhausted") {
		t.Error("expected 'exhausted' when remaining == 0")
	}
	if !strings.Contains(out, "resets") {
		t.Error("expected 'resets' when remaining == 0")
	}
}

func TestRenderHeader_Boundary100(t *testing.T) {
	// 100 is NOT < 100, so should take the green/default path (no resets shown).
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 100, Limit: 5000, ResetAt: time.Now()},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "100/5000") {
		t.Error("expected '100/5000' in header")
	}
	if strings.Contains(out, "resets") {
		t.Error("100 remaining should NOT show 'resets' (green path)")
	}
}

func TestRenderHeader_Boundary99(t *testing.T) {
	// 99 IS < 100, so should take the orange path (resets shown).
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 99, Limit: 5000, ResetAt: resetAt},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "99/5000") {
		t.Error("expected '99/5000' in header")
	}
	if !strings.Contains(out, "resets") {
		t.Error("99 remaining should show 'resets' (orange path)")
	}
}

func TestRenderHeader_GraphQLRateLimit(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{
			Remaining:        4500,
			Limit:            5000,
			ResetAt:          time.Now(),
			GraphQLRemaining: 4800,
			GraphQLLimit:     5000,
			GraphQLResetAt:   time.Now(),
		},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if !strings.Contains(out, "4500/5000") {
		t.Error("expected REST '4500/5000' in header")
	}
	if !strings.Contains(out, "GQL:") {
		t.Error("expected 'GQL:' in header when GraphQL limit > 0")
	}
	if !strings.Contains(out, "4800/5000") {
		t.Error("expected GraphQL '4800/5000' in header")
	}
}

func TestRenderHeader_GraphQLHiddenWhenZeroLimit(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 4500, Limit: 5000, ResetAt: time.Now()},
	}
	out := renderHeaderWithQueue(120, report, false, "", nil, 0, false)
	if strings.Contains(out, "GQL:") {
		t.Error("should not show 'GQL:' when GraphQLLimit is 0")
	}
}

func TestRenderHeader_NilReport(t *testing.T) {
	out := renderHeaderWithQueue(120, nil, true, "⣾", nil, 0, false)
	if !strings.Contains(out, "Loading") {
		t.Error("expected 'Loading' when report is nil")
	}
}

func TestRenderHeader_Loading(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
		RateLimit: &RateLimit{Remaining: 4000, Limit: 5000, ResetAt: time.Now()},
	}
	out := renderHeaderWithQueue(120, report, true, "⣾", nil, 0, false)
	if !strings.Contains(out, "refreshing") {
		t.Error("expected 'refreshing' when loading is true")
	}
}

func TestRenderHeader_QueueLoading(t *testing.T) {
	report := &AuditReport{
		Source:    "test",
		Timestamp: time.Now(),
	}
	queueLoading := map[string]bool{"openshift/velero": true}
	out := renderHeaderWithQueue(120, report, false, "⣾", queueLoading, 0, false)
	if !strings.Contains(out, "queue") {
		t.Error("expected 'queue' indicator when queue is loading")
	}
	if !strings.Contains(out, "velero") {
		t.Error("expected repo name in queue loading indicator")
	}
}
