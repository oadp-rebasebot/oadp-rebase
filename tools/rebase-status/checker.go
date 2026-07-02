package main

import (
	"fmt"
	"os"
	"sync"
)

// RunAllChecks runs every registered check against every repo in parallel.
// Each repo is checked concurrently; checks within a repo run sequentially
// (to avoid blasting the API with too many goroutines).
func RunAllChecks(specs []RepoSpec, checks []Check, client *GitHubClient) []RepoStatus {
	results := make([]RepoStatus, len(specs))
	var wg sync.WaitGroup

	for i := range specs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = runRepoChecks(specs[idx], checks, client)
		}(i)
	}

	wg.Wait()
	return results
}

// runRepoChecks runs all checks for a single repo.
func runRepoChecks(spec RepoSpec, checks []Check, client *GitHubClient) RepoStatus {
	status := RepoStatus{
		Spec:   spec,
		Checks: make(map[string]*CheckResult),
	}

	if spec.Skip {
		// Mark all checks as skipped
		for _, chk := range checks {
			status.Checks[chk.ID] = &CheckResult{StatusSkip, "", "SKIP_REPO=true"}
		}
		return status
	}

	for _, chk := range checks {
		result := safeRun(chk, client, &spec)
		status.Checks[chk.ID] = result

		// Collect issues
		switch result.Status {
		case StatusFail:
			msg := result.Detail
			if msg == "" {
				msg = chk.Header + " check failed"
			}
			status.Issues = append(status.Issues, Issue{
				Severity: "error",
				Repo:     spec.FullName(),
				Message:  msg,
			})
		case StatusWarn:
			msg := result.Detail
			if msg == "" {
				msg = chk.Header + " warning"
			}
			status.Issues = append(status.Issues, Issue{
				Severity: "warning",
				Repo:     spec.FullName(),
				Message:  msg,
			})
		}
	}

	// Propagate dep sync details from the store
	depSyncStoreMu.Lock()
	if syncs, ok := depSyncStore[spec.FullName()]; ok {
		status.DepSyncs = syncs
	}
	depSyncStoreMu.Unlock()

	// Propagate image info from the store
	imageStoreMu.Lock()
	if imgs, ok := imageStore[spec.FullName()]; ok {
		status.Images = imgs
	}
	imageStoreMu.Unlock()

	// Propagate Konflux info from the store
	konfluxStoreMu.Lock()
	if info, ok := konfluxStore[spec.FullName()]; ok {
		status.Konflux = info
	}
	konfluxStoreMu.Unlock()

	// Propagate ART configs from the store
	artConfigStoreMu.Lock()
	if cfgs, ok := artConfigStore[spec.FullName()]; ok {
		status.ArtConfigs = cfgs
	}
	artConfigStoreMu.Unlock()

	// Propagate open PR info from the store
	openPRStoreMu.Lock()
	if pr, ok := openPRStore[spec.FullName()]; ok {
		status.OpenPR = pr
	}
	openPRStoreMu.Unlock()

	// Propagate go.mod drift from the store
	gomodDriftStoreMu.Lock()
	if drifts, ok := gomodDriftStore[spec.FullName()]; ok {
		status.GoModDrifts = drifts
	}
	gomodDriftStoreMu.Unlock()

	// Propagate release data
	if currentReleaseData != nil {
		key := spec.FullName()
		status.ImageRefData = currentReleaseData.ImageRefsFor(key)
		if len(status.ArtConfigs) == 0 {
			status.ArtConfigs = currentReleaseData.ArtConfigsFor(key)
		}
	}

	return status
}

// safeRun executes a check with panic recovery.
func safeRun(chk Check, client *GitHubClient, spec *RepoSpec) (result *CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic in check %s for %s: %v\n", chk.ID, spec.FullName(), r)
			result = &CheckResult{StatusWarn, "ERR", fmt.Sprintf("panic: %v", r)}
		}
	}()
	return chk.Run(client, spec)
}
