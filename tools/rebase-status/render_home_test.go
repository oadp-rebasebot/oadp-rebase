package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestGithubAnchor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			":construction: Wave 4 — Downstream Controllers (3/4)",
			"construction-wave-4--downstream-controllers-34",
		},
		{
			":white_check_mark: Wave 1 — Independent Dependencies (5/5)",
			"white_check_mark-wave-1--independent-dependencies-55",
		},
		{
			"Simple Header",
			"simple-header",
		},
		{
			"UPPER case WITH numbers 123",
			"upper-case-with-numbers-123",
		},
	}
	for _, tt := range tests {
		got := githubAnchor(tt.input)
		if got != tt.want {
			t.Errorf("githubAnchor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDaysAgo(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{0, "today"},
		{1, "1 day ago"},
		{2, "2 days ago"},
		{30, "30 days ago"},
		{97, "97 days ago"},
	}
	for _, tt := range tests {
		got := formatDaysAgo(tt.days)
		if got != tt.want {
			t.Errorf("formatDaysAgo(%d) = %q, want %q", tt.days, got, tt.want)
		}
	}
}

func TestHomeScoreLine(t *testing.T) {
	tests := []struct {
		total, ready, errs, warns int
		want                      string
	}{
		{18, 14, 4, 1, "14/18 repos ready (77%) | 4 error(s) | 1 warning(s)"},
		{10, 10, 0, 0, "10/10 repos ready (100%)"},
		{0, 0, 0, 0, "0/0 repos ready"},
		{5, 3, 2, 0, "3/5 repos ready (60%) | 2 error(s)"},
		{8, 7, 0, 1, "7/8 repos ready (87%) | 1 warning(s)"},
	}
	for _, tt := range tests {
		got := homeScoreLine(tt.total, tt.ready, tt.errs, tt.warns)
		if got != tt.want {
			t.Errorf("homeScoreLine(%d, %d, %d, %d) = %q, want %q",
				tt.total, tt.ready, tt.errs, tt.warns, got, tt.want)
		}
	}
}

func TestComputeWaveStatuses(t *testing.T) {
	statuses := []RepoStatus{
		{Spec: RepoSpec{Wave: 1}, Issues: nil},
		{Spec: RepoSpec{Wave: 1}, Issues: []Issue{{Severity: "error"}}},
		{Spec: RepoSpec{Wave: 2}, Issues: nil},
		{Spec: RepoSpec{Wave: 2, Skip: true}},
	}

	ws := computeWaveStatuses(statuses)
	if len(ws) != 2 {
		t.Fatalf("got %d waves, want 2", len(ws))
	}
	if ws[0].Ready != 1 || ws[0].Total != 2 {
		t.Errorf("wave 1: ready=%d total=%d, want 1/2", ws[0].Ready, ws[0].Total)
	}
	if ws[1].Ready != 1 || ws[1].Total != 1 {
		t.Errorf("wave 2: ready=%d total=%d, want 1/1 (skip excluded)", ws[1].Ready, ws[1].Total)
	}
}

func TestCollectOpenPRs(t *testing.T) {
	pr1 := &OpenPRInfo{Number: 42, URL: "https://example.com/42", CreatedAt: time.Now()}
	pr2 := &OpenPRInfo{Number: 99, URL: "https://example.com/99", CreatedAt: time.Now()}
	statuses := []RepoStatus{
		{Spec: RepoSpec{Repo: "velero", Wave: 2}, OpenPR: pr1},
		{Spec: RepoSpec{Repo: "kopia", Wave: 1}},
		{Spec: RepoSpec{Repo: "oadp-operator", Wave: 3}, OpenPR: pr2},
	}

	prs := collectOpenPRs(statuses)
	if len(prs) != 2 {
		t.Fatalf("got %d open PRs, want 2", len(prs))
	}
	// Should be ordered by wave (1, 2, 3)
	if prs[0].Repo != "velero" || prs[1].Repo != "oadp-operator" {
		t.Errorf("unexpected order: %s, %s", prs[0].Repo, prs[1].Repo)
	}
}

func TestLatestOpenPR(t *testing.T) {
	now := time.Now()
	prs := []openPREntry{
		{Repo: "old", PR: &OpenPRInfo{CreatedAt: now.Add(-48 * time.Hour)}},
		{Repo: "newest", PR: &OpenPRInfo{CreatedAt: now}},
		{Repo: "middle", PR: &OpenPRInfo{CreatedAt: now.Add(-24 * time.Hour)}},
	}
	got := latestOpenPR(prs)
	if got == nil || got.Repo != "newest" {
		t.Errorf("latestOpenPR returned %v, want newest", got)
	}

	if latestOpenPR(nil) != nil {
		t.Error("latestOpenPR(nil) should return nil")
	}
}

func TestRenderHome(t *testing.T) {
	now := time.Now()
	branches := []BranchResult{
		{
			Branch: "oadp-1.6",
			Statuses: []RepoStatus{
				{
					Spec:   RepoSpec{Repo: "velero", Wave: 2, HasConfig: true},
					OpenPR: &OpenPRInfo{Number: 42, URL: "https://example.com/42", CreatedAt: now},
				},
				{
					Spec:   RepoSpec{Repo: "kopia", Wave: 1, HasConfig: true},
					Issues: []Issue{{Severity: "error", Message: "missing"}},
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches)
	out := buf.String()

	// Header
	if !strings.Contains(out, "# OADP Rebase Status") {
		t.Error("missing page title")
	}

	// Branch section
	if !strings.Contains(out, "## [oadp-1.6](Rebase-Status-oadp-1.6)") {
		t.Error("missing branch header")
	}

	// Score
	if !strings.Contains(out, "1/2 repos ready (50%)") {
		t.Error("missing or wrong score line")
	}

	// Latest PR with days-ago
	if !strings.Contains(out, "Latest PR:") || !strings.Contains(out, "velero #42") {
		t.Error("missing latest PR line")
	}
	if !strings.Contains(out, "**today**") {
		t.Error("missing days-ago in latest PR")
	}

	// Open PRs section
	if !strings.Contains(out, "**Open rebase PRs:**") {
		t.Error("missing open PRs section")
	}

	// Slack snippet
	if !strings.Contains(out, "<summary>Copy</summary>") {
		t.Error("missing Slack copy section")
	}
	if !strings.Contains(out, "velero #42") || !strings.Contains(out, "https://example.com/42") {
		t.Error("missing PR link in Slack snippet")
	}

	// Should NOT show TODO (configs exist)
	if strings.Contains(out, "TODO") {
		t.Error("should not show TODO when configs exist")
	}
}

func TestRenderHomeTODO(t *testing.T) {
	branches := []BranchResult{
		{
			Branch: "oadp-1.3",
			Statuses: []RepoStatus{
				{
					Spec:   RepoSpec{Repo: "velero", Wave: 2, HasConfig: false},
					Issues: []Issue{{Severity: "error", Message: "no config"}},
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches)
	out := buf.String()

	if !strings.Contains(out, "> **TODO:**") {
		t.Error("should show TODO when no configs exist")
	}
}
