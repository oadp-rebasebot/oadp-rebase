package main

import (
	"os"
	"path/filepath"
	"testing"
)

func findReposYAMLForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "repos.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repos.yaml not found")
		}
		dir = parent
	}
}

func TestLoadReposYAML(t *testing.T) {
	path := findReposYAMLForTest(t)
	if err := LoadReposYAML(path); err != nil {
		t.Fatalf("LoadReposYAML: %v", err)
	}

	if len(allRepos) == 0 {
		t.Fatal("allRepos is empty after loading")
	}
	if len(wavesMeta) == 0 {
		t.Fatal("wavesMeta is empty after loading")
	}
	if len(filenameToRepo) == 0 {
		t.Fatal("filenameToRepo is empty after loading")
	}
}

func TestRequiredFields(t *testing.T) {
	path := findReposYAMLForTest(t)
	if err := LoadReposYAML(path); err != nil {
		t.Fatalf("LoadReposYAML: %v", err)
	}

	for i, r := range allRepos {
		if r.Org == "" {
			t.Errorf("repo %d: missing org", i)
		}
		if r.Repo == "" {
			t.Errorf("repo %d: missing repo", i)
		}
		if r.Wave == 0 {
			t.Errorf("repo %d (%s/%s): missing wave", i, r.Org, r.Repo)
		}
	}
}

func TestConfigPrefixUnique(t *testing.T) {
	path := findReposYAMLForTest(t)
	if err := LoadReposYAML(path); err != nil {
		t.Fatalf("LoadReposYAML: %v", err)
	}

	seen := make(map[string]string)
	for prefix, entry := range filenameToRepo {
		key := entry.org + "/" + entry.repo
		if prev, ok := seen[prefix]; ok {
			t.Errorf("duplicate config_prefix %q: used by %s and %s", prefix, prev, key)
		}
		seen[prefix] = key
	}
}

func TestWaveReferences(t *testing.T) {
	path := findReposYAMLForTest(t)
	if err := LoadReposYAML(path); err != nil {
		t.Fatalf("LoadReposYAML: %v", err)
	}

	waveSet := make(map[int]bool)
	for _, w := range wavesMeta {
		waveSet[w.Number] = true
	}

	for _, r := range allRepos {
		if !waveSet[r.Wave] {
			t.Errorf("%s/%s references wave %d which is not defined in waves", r.Org, r.Repo, r.Wave)
		}
	}
}

func TestImagesValid(t *testing.T) {
	path := findReposYAMLForTest(t)
	if err := LoadReposYAML(path); err != nil {
		t.Fatalf("LoadReposYAML: %v", err)
	}

	for key, images := range repoImages {
		for i, img := range images {
			if img.Namespace == "" {
				t.Errorf("%s image %d: missing namespace", key, i)
			}
			if img.Repo == "" {
				t.Errorf("%s image %d: missing repo", key, i)
			}
			if img.Name == "" {
				t.Errorf("%s image %d: missing name", key, i)
			}
		}
	}
}
