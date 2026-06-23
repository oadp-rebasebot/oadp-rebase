package main

import (
	"testing"
)

func TestParseGitmodules(t *testing.T) {
	gitmodules := `[submodule "velero"]
	path = velero
	url = https://github.com/openshift/velero.git
	branch = oadp-1.6
[submodule "restic"]
	path = restic
	url = https://github.com/openshift/restic.git
	branch = oadp-1.6
[submodule "kopia"]
	path = kopia
	url = https://github.com/migtools/kopia.git
	branch = oadp-1.6
`
	subs := parseGitmodules(gitmodules)

	if len(subs) != 3 {
		t.Fatalf("expected 3 submodules, got %d", len(subs))
	}

	tests := []struct {
		path   string
		url    string
		branch string
	}{
		{"velero", "https://github.com/openshift/velero.git", "oadp-1.6"},
		{"restic", "https://github.com/openshift/restic.git", "oadp-1.6"},
		{"kopia", "https://github.com/migtools/kopia.git", "oadp-1.6"},
	}

	for _, tt := range tests {
		sub, ok := subs[tt.path]
		if !ok {
			t.Errorf("missing submodule %q", tt.path)
			continue
		}
		if sub.URL != tt.url {
			t.Errorf("submodule %q URL = %q, want %q", tt.path, sub.URL, tt.url)
		}
		if sub.Branch != tt.branch {
			t.Errorf("submodule %q Branch = %q, want %q", tt.path, sub.Branch, tt.branch)
		}
	}
}

func TestParseGitmodulesURLNormalization(t *testing.T) {
	gitmodules := `[submodule "kopia"]
	path = kopia
	url = https://github.com/migtools/kopia.git
	branch = oadp-1.6
`
	subs := parseGitmodules(gitmodules)
	sub := subs["kopia"]

	// URL should have .git stripped for matching against submoduleRepos
	if sub.URL != "https://github.com/migtools/kopia.git" {
		t.Errorf("unexpected URL %q", sub.URL)
	}
}

func TestParseSubmoduleDeps(t *testing.T) {
	gitmodules := `[submodule "velero"]
	path = velero
	url = https://github.com/openshift/velero.git
	branch = oadp-1.6
[submodule "kopia"]
	path = kopia
	url = https://github.com/migtools/kopia.git
	branch = oadp-1.6
`
	// Simulate git tree entries: submodules have type "commit"
	treeEntries := map[string]string{
		"velero": "abc123def456abc123def456abc123def456abc1",
		"kopia":  "def789abc012def789abc012def789abc012def7",
	}

	syncs := parseSubmoduleDeps(gitmodules, treeEntries, "openshift/oadp-must-gather")

	if len(syncs) != 2 {
		t.Fatalf("expected 2 submodule deps, got %d", len(syncs))
	}

	// Check velero
	found := false
	for _, s := range syncs {
		if s.Repo == "velero" {
			found = true
			if s.Org != "openshift" {
				t.Errorf("velero org = %q, want openshift", s.Org)
			}
			if s.HaveHash != "abc123def456" {
				t.Errorf("velero HaveHash = %q, want first 12 chars", s.HaveHash)
			}
		}
	}
	if !found {
		t.Error("velero not found in submodule deps")
	}

	// Check kopia
	found = false
	for _, s := range syncs {
		if s.Repo == "kopia" {
			found = true
			if s.Org != "migtools" {
				t.Errorf("kopia org = %q, want migtools", s.Org)
			}
		}
	}
	if !found {
		t.Error("kopia not found in submodule deps")
	}
}

func TestParseSubmoduleDepsSkipsUnknownRepos(t *testing.T) {
	gitmodules := `[submodule "some-external"]
	path = external
	url = https://github.com/random/repo.git
	branch = main
`
	treeEntries := map[string]string{
		"external": "abc123def456abc123def456abc123def456abc1",
	}

	syncs := parseSubmoduleDeps(gitmodules, treeEntries, "openshift/something")

	if len(syncs) != 0 {
		t.Errorf("expected 0 deps for unknown submodule, got %d", len(syncs))
	}
}

func TestParseSubmoduleDepsSkipsSelfReference(t *testing.T) {
	gitmodules := `[submodule "velero"]
	path = velero
	url = https://github.com/openshift/velero.git
	branch = oadp-1.6
`
	treeEntries := map[string]string{
		"velero": "abc123def456abc123def456abc123def456abc1",
	}

	// Self-repo is openshift/velero — should skip
	syncs := parseSubmoduleDeps(gitmodules, treeEntries, "openshift/velero")

	if len(syncs) != 0 {
		t.Errorf("expected 0 deps (self-reference), got %d", len(syncs))
	}
}
