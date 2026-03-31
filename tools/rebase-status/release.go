package main

import (
	"fmt"
	"sync"
)

// currentReleaseData holds the release-level metadata fetched once per run.
// Set in main() before running checks, available to all check functions.
var currentReleaseData *ReleaseData

// FetchReleaseData fetches image-references and ocp-build-data for a branch,
// then builds the cross-reference maps that link image entries to source repos.
func FetchReleaseData(client *GitHubClient, branch string) (*ReleaseData, error) {
	rd := &ReleaseData{
		repoImageRefs:  make(map[string][]*ImageRefEntry),
		repoArtConfigs: make(map[string][]*ArtBuildConfig),
	}

	var wg sync.WaitGroup
	var imgRefsErr, artErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		refs, err := FetchImageReferences(client, branch)
		if err != nil {
			imgRefsErr = err
			return
		}
		if refs != nil {
			rd.ImageRefs = refs
			rd.HasImageRefs = true
		}
	}()

	go func() {
		defer wg.Done()
		configs, err := FetchArtConfigs(client, branch)
		if err != nil {
			artErr = err
			return
		}
		if configs != nil {
			rd.ArtConfigs = configs
			rd.HasArtBranch = true
		}
	}()

	wg.Wait()

	if imgRefsErr != nil {
		return rd, fmt.Errorf("image-references: %w", imgRefsErr)
	}
	if artErr != nil {
		return rd, fmt.Errorf("art configs: %w", artErr)
	}

	// Build ART config map: source "org/repo" -> configs (multiple per repo possible)
	for _, cfg := range rd.ArtConfigs {
		orgRepo := cfg.SourceOrgRepo()
		if orgRepo != "" {
			rd.repoArtConfigs[orgRepo] = append(rd.repoArtConfigs[orgRepo], cfg)
		}
	}

	// Build image-references map using ART configs for definitive mapping
	artNameToRepo := make(map[string]string)
	for _, cfg := range rd.ArtConfigs {
		artNameToRepo[cfg.ARTName()] = cfg.SourceOrgRepo()
	}

	for i := range rd.ImageRefs {
		ref := &rd.ImageRefs[i]
		var orgRepo string

		// Try ART config mapping first (definitive — from ocp-build-data source URL)
		if repo, ok := artNameToRepo[ref.ARTName]; ok {
			orgRepo = repo
		} else {
			// Fall back to quay repo name matching
			orgRepo = findRepoByQuayName(ref.QuayRepo)
		}

		if orgRepo != "" {
			rd.repoImageRefs[orgRepo] = append(rd.repoImageRefs[orgRepo], ref)
		}
	}

	return rd, nil
}

// findRepoByQuayName matches a quay repo name back to a source "org/repo"
// using the hardcoded repoImages map and the allRepos catalog.
func findRepoByQuayName(quayRepo string) string {
	// Check hardcoded repoImages (reverse lookup)
	for orgRepo, images := range repoImages {
		for _, img := range images {
			if img.Repo == quayRepo {
				return orgRepo
			}
		}
	}
	// Try direct name match against allRepos
	for _, def := range allRepos {
		if def.Repo == quayRepo {
			return def.Org + "/" + def.Repo
		}
	}
	return ""
}

// repoProducesImages returns true if a repo is expected to produce container images.
func repoProducesImages(orgRepo string) bool {
	if _, ok := repoImages[orgRepo]; ok {
		return true
	}
	if currentReleaseData != nil {
		if _, ok := currentReleaseData.repoImageRefs[orgRepo]; ok {
			return true
		}
		if _, ok := currentReleaseData.repoArtConfigs[orgRepo]; ok {
			return true
		}
	}
	return false
}

// imageTag returns the expected Quay image tag for a branch.
func imageTag(branch string) string {
	if branch == "oadp-dev" || branch == "main" {
		return "latest"
	}
	return branch
}
