package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSemverCompare(t *testing.T) {
	// Build the binary once for all subtests.
	bin := filepath.Join(t.TempDir(), "semver-compare")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir, _ = os.Getwd()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	tests := []struct {
		name string
		a, b string
		want string
	}{
		// Basic semver
		{"higher minor", "v0.27.0", "v0.28.0", "v0.28.0"},
		{"higher patch", "v1.2.3", "v1.2.4", "v1.2.4"},
		{"equal", "v1.0.0", "v1.0.0", "v1.0.0"},

		// Pre-release: release > pre-release (the sort -V bug)
		{"release beats rc", "v1.2.3", "v1.2.3-rc.1", "v1.2.3"},
		{"release beats pseudo", "v1.29.1", "v1.29.1-0.20250403044401-2c4cce9a3a42", "v1.29.1"},

		// Pseudo-versions: later timestamp wins
		{"pseudo timestamp order", "v0.0.0-20240101120000-abcdef123456", "v0.0.0-20240601120000-fedcba654321", "v0.0.0-20240601120000-fedcba654321"},

		// Tagged vs v0.0.0 pseudo-version
		{"tagged beats v0 pseudo", "v1.2.3", "v0.0.0-20240601120000-fedcba654321", "v1.2.3"},

		// +incompatible
		{"incompatible", "v2.0.0+incompatible", "v2.1.0+incompatible", "v2.1.0+incompatible"},

		// Pseudo-version based on a higher tag base
		{"pseudo higher base", "v1.29.0", "v1.29.1-0.20250403044401-2c4cce9a3a42", "v1.29.1-0.20250403044401-2c4cce9a3a42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command(bin, tc.a, tc.b).Output()
			if err != nil {
				t.Fatalf("semver-compare %s %s failed: %v", tc.a, tc.b, err)
			}
			got := string(out[:len(out)-1]) // trim newline
			if got != tc.want {
				t.Errorf("semver-compare %s %s = %s, want %s", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
