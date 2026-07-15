package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCountTriggeredFromLog(t *testing.T) {
	logContent := `# Auto Rebase Log

## 2026-07-15T18:00:00Z

**Run:** [#10](https://example.com/runs/10)
**Triggered by:** weshayutin (workflow_dispatch)
**Branches:** ` + "`oadp-1.6`" + `
**Dry run:** false
**Overall result:** success

| Target | Branch | Reason | Status | PR |
|--------|--------|--------|--------|----|
| ` + "`velero-oadp-1.6`" + ` | oadp-1.6 | upstream tag | ✅ Success | — |

---

## 2026-07-14T12:00:00Z

**Run:** [#9](https://example.com/runs/9)
**Triggered by:** weshayutin (workflow_dispatch)
**Branches:** ` + "`oadp-1.6 oadp-1.4`" + `
**Dry run:** false
**Overall result:** failure

---

## 2026-07-13T10:00:00Z

**Run:** [#8](https://example.com/runs/8)
**Triggered by:** weshayutin (workflow_dispatch)
**Branches:** ` + "`oadp-1.6`" + `
**Dry run:** true
**Overall result:** success

---

## 2026-07-12T08:00:00Z

**Run:** [#7](https://example.com/runs/7)
**Triggered by:** weshayutin (workflow_dispatch)
**Branches:** ` + "`oadp-1.4`" + `
**Dry run:** false
**Overall result:** success

---

## 2026-07-01T06:00:00Z

**Run:** [#6](https://example.com/runs/6)
**Triggered by:** weshayutin (workflow_dispatch)
**Branches:** ` + "`oadp-1.6`" + `
**Dry run:** false
**Overall result:** success

---
`
	dir := t.TempDir()
	logPath := filepath.Join(dir, "Auto-Rebase-V2-Log.md")
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		branch string
		since  time.Time
		want   int
	}{
		{
			name:   "oadp-1.6 since epoch counts non-dry-run entries",
			branch: "oadp-1.6",
			since:  time.Time{},
			want:   3, // #10, #9 (multi-branch), #6 — #8 excluded (dry run)
		},
		{
			name:   "oadp-1.4 since epoch",
			branch: "oadp-1.4",
			since:  time.Time{},
			want:   2, // #9 (multi-branch), #7
		},
		{
			name:   "oadp-1.6 since July 14 (filters out older entries)",
			branch: "oadp-1.6",
			since:  time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
			want:   2, // #10, #9
		},
		{
			name:   "oadp-1.5 has no entries",
			branch: "oadp-1.5",
			since:  time.Time{},
			want:   0,
		},
		{
			name:   "since future returns 0",
			branch: "oadp-1.6",
			since:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := countTriggeredFromLog(logPath, tt.branch, tt.since)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("countTriggeredFromLog(%q, %v) = %d, want %d", tt.branch, tt.since, got, tt.want)
			}
		})
	}
}

func TestCountTriggeredFromLog_MissingFile(t *testing.T) {
	got, err := countTriggeredFromLog("/nonexistent/path.md", "oadp-1.6", time.Time{})
	if err != nil {
		t.Fatalf("missing file should return 0, not error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 for missing file, got %d", got)
	}
}

func TestCountTriggeredFromLog_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "Auto-Rebase-V2-Log.md")
	if err := os.WriteFile(logPath, []byte("# Auto Rebase Log\n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := countTriggeredFromLog(logPath, "oadp-1.6", time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 for empty log, got %d", got)
	}
}
