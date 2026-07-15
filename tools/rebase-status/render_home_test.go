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
		tally                     *PRTallyResult
		want                      string
	}{
		{18, 14, 4, 1, nil, "14/18 repos ready (77%) | 4 error(s) | 1 warning(s)"},
		{10, 10, 0, 0, nil, "10/10 repos ready (100%)"},
		{0, 0, 0, 0, nil, "0/0 repos ready"},
		{5, 3, 2, 0, nil, "3/5 repos ready (60%) | 2 error(s)"},
		{8, 7, 0, 1, nil, "7/8 repos ready (87%) | 1 warning(s)"},
		{18, 3, 15, 0, &PRTallyResult{Triggered: 20, Opened: 12, Merged: 8}, "3/18 repos ready (16%) | 15 error(s) | :mailbox_with_mail: bot triggered 20 times, 12 opened prs, 8 merged prs"},
		{18, 18, 0, 0, &PRTallyResult{Triggered: 0, Opened: 0, Merged: 0}, "18/18 repos ready (100%) | :mailbox_with_mail: bot triggered 0 times, 0 opened prs, 0 merged prs"},
	}
	for _, tt := range tests {
		got := homeScoreLine(tt.total, tt.ready, tt.errs, tt.warns, tt.tally)
		if got != tt.want {
			t.Errorf("homeScoreLine(%d, %d, %d, %d, %v) = %q, want %q",
				tt.total, tt.ready, tt.errs, tt.warns, tt.tally, got, tt.want)
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

func TestRenderHomeWithTally(t *testing.T) {
	now := time.Now()
	branches := []BranchResult{
		{
			Branch: "oadp-1.6",
			Statuses: []RepoStatus{
				{
					Spec:   RepoSpec{Repo: "velero", Wave: 2, HasConfig: true},
					Issues: []Issue{{Severity: "error", Message: "out of sync"}},
				},
				{
					Spec:   RepoSpec{Repo: "kopia", Wave: 1, HasConfig: true},
					Issues: []Issue{{Severity: "error", Message: "missing"}},
				},
			},
			Tally: &PRTallyResult{Triggered: 10, Opened: 5, Merged: 3, ResetAt: now.Add(-72 * time.Hour)},
		},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches)
	out := buf.String()

	if !strings.Contains(out, ":mailbox_with_mail: bot triggered 10 times, 5 opened prs, 3 merged prs") {
		t.Error("missing tally in score line")
	}
	// Tally should also appear in the Slack copy snippet
	if count := strings.Count(out, ":mailbox_with_mail: bot triggered 10 times, 5 opened prs, 3 merged prs"); count < 2 {
		t.Errorf("tally should appear in both display and Slack copy, found %d times", count)
	}
}

func TestRenderHomeNoTally(t *testing.T) {
	branches := []BranchResult{
		{
			Branch: "oadp-1.6",
			Statuses: []RepoStatus{
				{Spec: RepoSpec{Repo: "velero", Wave: 2, HasConfig: true}},
			},
			Tally: nil,
		},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches)
	out := buf.String()

	if strings.Contains(out, "mailbox_with_mail") {
		t.Error("should not show tally when Tally is nil")
	}
}

func TestRenderHomeCycleHistory(t *testing.T) {
	branches := []BranchResult{
		{
			Branch: "oadp-1.6",
			Statuses: []RepoStatus{
				{Spec: RepoSpec{Repo: "velero", Wave: 2, HasConfig: true}},
			},
			Tally: &PRTallyResult{Triggered: 5, Opened: 3, Merged: 3},
		},
	}
	tallies := map[string]*PRTally{
		"oadp-1.6": {
			ResetAt: time.Now(),
			History: []PRCycleHistory{
				{Date: "2026-06-01", Score: "18/18 repos ready (100%)", Triggered: 25, Opened: 18, Merged: 18},
				{Date: "2026-07-01", Score: "18/18 repos ready (100%)", Triggered: 30, Opened: 20, Merged: 20},
			},
		},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches, tallies)
	out := buf.String()

	if !strings.Contains(out, "## Completed Rebase Cycles") {
		t.Error("missing history section header")
	}
	if !strings.Contains(out, "### oadp-1.6") {
		t.Error("missing branch history header")
	}
	if !strings.Contains(out, "| 2026-07-01 |") {
		t.Error("missing most recent history entry")
	}
	if !strings.Contains(out, "| 2026-06-01 |") {
		t.Error("missing older history entry")
	}
	// Most recent should appear first (before older entry in the output)
	recentIdx := strings.Index(out, "2026-07-01")
	olderIdx := strings.Index(out, "2026-06-01")
	if recentIdx > olderIdx {
		t.Error("history should show most recent first")
	}
}

func TestRenderHomeNoHistory(t *testing.T) {
	branches := []BranchResult{
		{
			Branch: "oadp-1.6",
			Statuses: []RepoStatus{
				{Spec: RepoSpec{Repo: "velero", Wave: 2, HasConfig: true}},
			},
		},
	}
	tallies := map[string]*PRTally{
		"oadp-1.6": {ResetAt: time.Now()},
	}

	var buf bytes.Buffer
	RenderHome(&buf, branches, tallies)
	out := buf.String()

	if strings.Contains(out, "Completed Rebase Cycles") {
		t.Error("should not render history section when no history exists")
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
