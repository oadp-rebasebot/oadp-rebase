package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type yamlRepoConfig struct {
	Waves map[int]string  `yaml:"waves"`
	Repos []yamlRepoDef   `yaml:"repos"`
}

type yamlRepoDef struct {
	Org          string          `yaml:"org"`
	Repo         string          `yaml:"repo"`
	Wave         int             `yaml:"wave"`
	ConfigPrefix string          `yaml:"config_prefix"`
	MainOnly     bool            `yaml:"main_only,omitempty"`
	NoRebase     bool            `yaml:"no_rebase,omitempty"`
	Branch       string          `yaml:"branch,omitempty"`
	MinBranch    string          `yaml:"min_branch,omitempty"`
	MaxBranch    string          `yaml:"max_branch,omitempty"`
	DevBranch    string          `yaml:"dev_branch,omitempty"`
	Images       []yamlQuayImage `yaml:"images,omitempty"`
}

type yamlQuayImage struct {
	Namespace string `yaml:"namespace"`
	Repo      string `yaml:"repo"`
	Name      string `yaml:"name"`
}

// FindReposYAML walks up from cwd looking for repos.yaml.
func FindReposYAML() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "repos.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("repos.yaml not found (searched from cwd upward)")
}

// LoadReposYAML parses repos.yaml and populates the package-level registry variables.
func LoadReposYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading repos.yaml: %w", err)
	}

	var cfg yamlRepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing repos.yaml: %w", err)
	}

	wavesMeta = nil
	for num, name := range cfg.Waves {
		wavesMeta = append(wavesMeta, WaveInfo{Number: num, Name: name})
	}
	sort.Slice(wavesMeta, func(i, j int) bool {
		return wavesMeta[i].Number < wavesMeta[j].Number
	})

	allRepos = nil
	for _, r := range cfg.Repos {
		allRepos = append(allRepos, RepoDef{
			Org:       r.Org,
			Repo:      r.Repo,
			Wave:      r.Wave,
			MainOnly:  r.MainOnly,
			NoRebase:  r.NoRebase,
			Branch:    r.Branch,
			MinBranch: r.MinBranch,
			MaxBranch: r.MaxBranch,
			DevBranch: r.DevBranch,
		})
	}

	repoImages = make(map[string][]QuayImage)
	for _, r := range cfg.Repos {
		if len(r.Images) == 0 {
			continue
		}
		key := r.Org + "/" + r.Repo
		for _, img := range r.Images {
			repoImages[key] = append(repoImages[key], QuayImage{
				Namespace: img.Namespace,
				Repo:      img.Repo,
				Name:      img.Name,
			})
		}
	}

	filenameToRepo = make(map[string]struct{ org, repo string })
	for _, r := range cfg.Repos {
		filenameToRepo[r.ConfigPrefix] = struct{ org, repo string }{r.Org, r.Repo}
	}

	return nil
}
