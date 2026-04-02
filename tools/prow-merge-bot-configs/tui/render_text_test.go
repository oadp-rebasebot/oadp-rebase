package main

import (
	"strings"
	"testing"
	"time"
)

func newTestReport(rl *RateLimit) *AuditReport {
	return &AuditReport{
		Source:    "test @ main",
		Timestamp: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		RateLimit: rl,
		Groups: []RepoGroup{
			{
				Name:        "Test Group",
				Type:        "upstream-rebase",
				Description: "Test repos",
				Repos: []RepoAudit{
					{
						Name:      "openshift/velero",
						HasConfig: true,
						Fields: map[string]string{
							"allow_force_pushes": "true",
						},
						Plugins: PluginStatus{HasApprovePlugin: true, HasLgtmSection: true},
						Findings: []Finding{
							{Severity: SeverityOK, Field: "allow_force_pushes", Message: "allow_force_pushes=true"},
						},
					},
				},
			},
		},
		Summary: Summary{Issues: 0, Warnings: 1, Info: 2},
	}
}

func TestRenderText_WithRateLimit(t *testing.T) {
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := newTestReport(&RateLimit{Remaining: 4000, Limit: 5000, ResetAt: resetAt})
	out := RenderText(report)
	if !strings.Contains(out, "4000/5000 remaining") {
		t.Error("expected '4000/5000 remaining' in text output")
	}
	if !strings.Contains(out, "14:00 UTC") {
		t.Error("expected reset time in text output")
	}
}

func TestRenderText_NilRateLimit(t *testing.T) {
	report := newTestReport(nil)
	out := RenderText(report)
	if strings.Contains(out, "API:") {
		t.Error("expected no 'API:' when RateLimit is nil")
	}
}

func TestRenderText_NilReport(t *testing.T) {
	out := RenderText(nil)
	if !strings.Contains(out, "No audit data") {
		t.Error("expected 'No audit data' for nil report")
	}
}

func TestRenderMarkdown_WithRateLimit(t *testing.T) {
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := newTestReport(&RateLimit{Remaining: 4000, Limit: 5000, ResetAt: resetAt})
	out := RenderMarkdown(report)
	if !strings.Contains(out, "**API Rate Limit:** 4000/5000 remaining") {
		t.Error("expected markdown rate limit in output")
	}
}

func TestRenderMarkdown_NilRateLimit(t *testing.T) {
	report := newTestReport(nil)
	out := RenderMarkdown(report)
	if strings.Contains(out, "API Rate Limit") {
		t.Error("expected no rate limit section when nil")
	}
}

func TestRenderTextPlain_WithRateLimit(t *testing.T) {
	resetAt := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	report := newTestReport(&RateLimit{Remaining: 4000, Limit: 5000, ResetAt: resetAt})
	out := RenderTextPlain(report)
	if !strings.Contains(out, "4000/5000") {
		t.Error("expected rate limit in plain text output")
	}
}

func TestRenderTextPlain_NilReport(t *testing.T) {
	out := RenderTextPlain(nil)
	if out != "" {
		t.Errorf("expected empty string for nil report, got %q", out)
	}
}

func TestRenderJSON_IncludesRateLimit(t *testing.T) {
	report := newTestReport(&RateLimit{Remaining: 4000, Limit: 5000, ResetAt: time.Now()})
	out, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"remaining": 4000`) {
		t.Error("expected remaining in JSON output")
	}
	if !strings.Contains(out, `"limit": 5000`) {
		t.Error("expected limit in JSON output")
	}
}

func TestRenderJSON_NilRateLimit(t *testing.T) {
	report := newTestReport(nil)
	out, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// rate_limit should be omitted (omitempty)
	if strings.Contains(out, `"rate_limit"`) {
		t.Error("expected rate_limit to be omitted when nil")
	}
}

func TestRenderText_Summary(t *testing.T) {
	report := newTestReport(nil)
	report.Summary = Summary{Issues: 3, Warnings: 5, Info: 12}
	out := RenderText(report)
	if !strings.Contains(out, "3") {
		t.Error("expected issue count in summary")
	}
	if !strings.Contains(out, "5") {
		t.Error("expected warning count in summary")
	}
}
