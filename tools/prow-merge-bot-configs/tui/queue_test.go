package main

import (
	"encoding/json"
	"testing"
)

func TestNormalizeChecks_StatusContext(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"context":"ci/prow/test","state":"SUCCESS","targetUrl":"https://example.com"}`),
	}
	checks := normalizeChecks(raw)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Name != "ci/prow/test" {
		t.Errorf("Name = %q, want ci/prow/test", checks[0].Name)
	}
	if checks[0].State != "SUCCESS" {
		t.Errorf("State = %q, want SUCCESS", checks[0].State)
	}
	if checks[0].URL != "https://example.com" {
		t.Errorf("URL = %q, want https://example.com", checks[0].URL)
	}
}

func TestNormalizeChecks_CheckRun(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"name":"e2e-test","status":"COMPLETED","conclusion":"FAILURE","detailsUrl":"https://ci.example.com"}`),
	}
	checks := normalizeChecks(raw)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Name != "e2e-test" {
		t.Errorf("Name = %q, want e2e-test", checks[0].Name)
	}
	if checks[0].State != "FAILURE" {
		t.Errorf("State = %q, want FAILURE", checks[0].State)
	}
}

func TestNormalizeChecks_Mixed(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"context":"tide","state":"success"}`),
		json.RawMessage(`{"name":"unit-tests","status":"COMPLETED","conclusion":"SUCCESS"}`),
		json.RawMessage(`{"name":"lint","status":"IN_PROGRESS"}`),
	}
	checks := normalizeChecks(raw)
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}
	if checks[0].State != "SUCCESS" {
		t.Errorf("tide state = %q, want SUCCESS", checks[0].State)
	}
	if checks[2].State != "IN_PROGRESS" {
		t.Errorf("lint state = %q, want IN_PROGRESS", checks[2].State)
	}
}

func TestNormalizeChecks_InvalidJSON(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{invalid`),
	}
	checks := normalizeChecks(raw)
	if len(checks) != 0 {
		t.Errorf("expected 0 checks for invalid JSON, got %d", len(checks))
	}
}

func TestNormalizeCheckRunState(t *testing.T) {
	tests := []struct {
		status, conclusion, want string
	}{
		{"COMPLETED", "SUCCESS", "SUCCESS"},
		{"COMPLETED", "FAILURE", "FAILURE"},
		{"COMPLETED", "", "UNKNOWN"},
		{"IN_PROGRESS", "", "IN_PROGRESS"},
		{"QUEUED", "", "PENDING"},
		{"", "", "PENDING"},
	}
	for _, tt := range tests {
		got := normalizeCheckRunState(tt.status, tt.conclusion)
		if got != tt.want {
			t.Errorf("normalizeCheckRunState(%q, %q) = %q, want %q", tt.status, tt.conclusion, got, tt.want)
		}
	}
}

func TestNormalizeStatusContextState(t *testing.T) {
	tests := []struct {
		state, want string
	}{
		{"SUCCESS", "SUCCESS"},
		{"FAILURE", "FAILURE"},
		{"ERROR", "ERROR"},
		{"success", "SUCCESS"},
		{"pending", "PENDING"},
		{"unknown", "PENDING"},
	}
	for _, tt := range tests {
		got := normalizeStatusContextState(tt.state)
		if got != tt.want {
			t.Errorf("normalizeStatusContextState(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCountApprovals(t *testing.T) {
	tests := []struct {
		name      string
		reviews   []ghReview
		wantCount int
		wantNames []string
	}{
		{
			name:      "no reviews",
			reviews:   nil,
			wantCount: 0,
		},
		{
			name: "one approval",
			reviews: []ghReview{
				{Author: ghAuthor{Login: "alice"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
			},
			wantCount: 1,
			wantNames: []string{"alice"},
		},
		{
			name: "approval then changes requested",
			reviews: []ghReview{
				{Author: ghAuthor{Login: "alice"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
				{Author: ghAuthor{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: "2024-01-02T00:00:00Z"},
			},
			wantCount: 0,
		},
		{
			name: "changes requested then re-approved",
			reviews: []ghReview{
				{Author: ghAuthor{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: "2024-01-01T00:00:00Z"},
				{Author: ghAuthor{Login: "alice"}, State: "APPROVED", SubmittedAt: "2024-01-02T00:00:00Z"},
			},
			wantCount: 1,
			wantNames: []string{"alice"},
		},
		{
			name: "multiple reviewers",
			reviews: []ghReview{
				{Author: ghAuthor{Login: "alice"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
				{Author: ghAuthor{Login: "bob"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
			},
			wantCount: 2,
			wantNames: []string{"alice", "bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, names := countApprovals(tt.reviews)
			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}
			if tt.wantNames != nil {
				if len(names) != len(tt.wantNames) {
					t.Errorf("names = %v, want %v", names, tt.wantNames)
				}
				for i, name := range tt.wantNames {
					if i < len(names) && names[i] != name {
						t.Errorf("names[%d] = %q, want %q", i, names[i], name)
					}
				}
			}
		})
	}
}

func TestDetectBlockingLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []ghLabel
		want   int
	}{
		{"no labels", nil, 0},
		{"non-blocking labels", []ghLabel{{Name: "approved"}, {Name: "lgtm"}}, 0},
		{"one blocker", []ghLabel{{Name: "do-not-merge/hold"}, {Name: "approved"}}, 1},
		{"multiple blockers", []ghLabel{
			{Name: "do-not-merge/hold"},
			{Name: "needs-rebase"},
			{Name: "approved"},
		}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockers := detectBlockingLabels(tt.labels)
			if len(blockers) != tt.want {
				t.Errorf("got %d blockers, want %d", len(blockers), tt.want)
			}
		})
	}
}

func TestCategorizeChecks(t *testing.T) {
	checks := []normalizedCheck{
		{Name: "tide", State: "SUCCESS"},
		{Name: "e2e", State: "FAILURE", URL: "https://ci.example.com/1"},
		{Name: "lint", State: "SUCCESS"},
		{Name: "unit", State: "PENDING"},
		{Name: "build", State: "IN_PROGRESS"},
		{Name: "sec", State: "ERROR"},
	}

	cs := categorizeChecks(checks)

	if cs.TideState != "SUCCESS" {
		t.Errorf("TideState = %q, want SUCCESS", cs.TideState)
	}
	if len(cs.Failing) != 1 || cs.Failing[0].Name != "e2e" {
		t.Errorf("Failing = %v, want [e2e]", cs.Failing)
	}
	if len(cs.Pending) != 1 || cs.Pending[0].Name != "unit" {
		t.Errorf("Pending = %v, want [unit]", cs.Pending)
	}
	if len(cs.InProgress) != 1 || cs.InProgress[0].Name != "build" {
		t.Errorf("InProgress = %v, want [build]", cs.InProgress)
	}
	if len(cs.Errored) != 1 || cs.Errored[0].Name != "sec" {
		t.Errorf("Errored = %v, want [sec]", cs.Errored)
	}
}

func TestCategorizeChecks_NoTide(t *testing.T) {
	checks := []normalizedCheck{
		{Name: "unit", State: "SUCCESS"},
	}
	cs := categorizeChecks(checks)
	if cs.TideState != "NOT_REPORTED" {
		t.Errorf("TideState = %q, want NOT_REPORTED", cs.TideState)
	}
}

func TestSplitRepo(t *testing.T) {
	org, repo := splitRepo("openshift/velero")
	if org != "openshift" || repo != "velero" {
		t.Errorf("splitRepo = (%q, %q), want (openshift, velero)", org, repo)
	}

	org, repo = splitRepo("noslash")
	if org != "noslash" || repo != "" {
		t.Errorf("splitRepo(noslash) = (%q, %q), want (noslash, \"\")", org, repo)
	}
}
