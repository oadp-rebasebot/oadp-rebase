package main

import (
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// artYAML mirrors the ocp-build-data image config YAML structure.
type artYAML struct {
	Name    string `yaml:"name"`
	Content struct {
		Source struct {
			Dockerfile string `yaml:"dockerfile"`
			Git        struct {
				Branch struct {
					Target string `yaml:"target"`
				} `yaml:"branch"`
				URL string `yaml:"url"`
				Web string `yaml:"web"`
			} `yaml:"git"`
		} `yaml:"source"`
	} `yaml:"content"`
	Distgit struct {
		Component string `yaml:"component"`
	} `yaml:"distgit"`
	Dependents []string `yaml:"dependents"`
}

// ARTName returns the image name without the namespace prefix.
// e.g. "oadp/oadp-velero-plugin-for-gcp-rhel9" -> "oadp-velero-plugin-for-gcp-rhel9"
func (a *ArtBuildConfig) ARTName() string {
	if idx := strings.LastIndex(a.Name, "/"); idx >= 0 {
		return a.Name[idx+1:]
	}
	return a.Name
}

// SourceOrgRepo extracts "org/repo" from the web URL.
// e.g. "https://github.com/openshift/velero-plugin-for-gcp" -> "openshift/velero-plugin-for-gcp"
func (a *ArtBuildConfig) SourceOrgRepo() string {
	web := strings.TrimPrefix(a.SourceWeb, "https://github.com/")
	web = strings.TrimSuffix(web, "/")
	return web
}

// FetchArtConfigs fetches and parses all ocp-build-data image configs for a branch.
// Returns nil, nil if the branch doesn't exist in ocp-build-data.
func FetchArtConfigs(client *GitHubClient, branch string) ([]*ArtBuildConfig, error) {
	exists, err := client.BranchExists("openshift-eng", "ocp-build-data", branch)
	if err != nil {
		return nil, fmt.Errorf("checking ocp-build-data branch: %w", err)
	}
	if !exists {
		return nil, nil
	}

	files, err := client.DirListingRef("openshift-eng", "ocp-build-data", "images", branch)
	if err != nil {
		return nil, fmt.Errorf("listing ocp-build-data images: %w", err)
	}
	if files == nil {
		return nil, nil
	}

	// Fetch all YAML files in parallel
	type fetchResult struct {
		cfg *ArtBuildConfig
	}
	ch := make(chan fetchResult, len(files))
	var wg sync.WaitGroup

	for _, filename := range files {
		if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
			continue
		}
		wg.Add(1)
		go func(fname string) {
			defer wg.Done()
			content, err := client.FileContent("openshift-eng", "ocp-build-data", "images/"+fname, branch)
			if err != nil || content == nil {
				return
			}
			cfg := parseArtYAML(string(content))
			if cfg != nil {
				cfg.Filename = fname
				ch <- fetchResult{cfg}
			}
		}(filename)
	}

	wg.Wait()
	close(ch)

	var configs []*ArtBuildConfig
	for r := range ch {
		configs = append(configs, r.cfg)
	}

	return configs, nil
}

// parseArtYAML parses an ocp-build-data image config YAML into ArtBuildConfig.
func parseArtYAML(content string) *ArtBuildConfig {
	var y artYAML
	if err := yaml.Unmarshal([]byte(content), &y); err != nil {
		return nil
	}

	return &ArtBuildConfig{
		Name:         y.Name,
		SourceWeb:    y.Content.Source.Git.Web,
		SourceURL:    y.Content.Source.Git.URL,
		BranchTarget: y.Content.Source.Git.Branch.Target,
		Dockerfile:   y.Content.Source.Dockerfile,
		Component:    y.Distgit.Component,
		Dependents:   y.Dependents,
	}
}
