package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// DefaultChecks is the ordered list of checks to run.
// To add a new check: define a CheckFunc, add a Check entry here.
var DefaultChecks = []Check{
	{ID: "open_pr", Header: "PR", Run: checkOpenPR},
	{ID: "config", Header: "Rebase Cfg", Run: checkConfig},
	{ID: "rebasebot", Header: "Rebase", Run: checkRebasebotBranch},
	{ID: "go_version", Header: "Go", Run: checkGoVersion},
	{ID: "ci_config", Header: "Prow Cfg", Run: checkCIConfig},
	{ID: "dep_sync", Header: "Deps", Run: checkDepSync},
	{ID: "gomod_drift", Header: "Drift", Run: checkGoModDrift},
	{ID: "konflux", Header: "Konflux", Run: checkKonflux},
	{ID: "upstream_image", Header: "Quay", Run: checkUpstreamImage},
	{ID: "art_config", Header: "ART Cfg", Run: checkArtConfig},
	{ID: "productized", Header: "Bundle", Run: checkProductized},
}

// downstreamModules maps Go module paths of downstream forks to their
// GitHub org/repo. Used to detect internal dependencies in go.mod.
// Both the downstream module path AND the upstream module path (for replaces)
// should be listed if they differ.
// Add new entries here when onboarding repos with cross-dependencies.
var downstreamModules = map[string]struct{ org, repo string }{
	"github.com/openshift/velero":                      {"openshift", "velero"},
	"github.com/migtools/kopia":                        {"migtools", "kopia"},
	"github.com/openshift/oadp-operator":               {"openshift", "oadp-operator"},
	"github.com/migtools/oadp-non-admin":               {"migtools", "oadp-non-admin"},
	"github.com/migtools/kubevirt-datamover-controller": {"migtools", "kubevirt-datamover-controller"},
	"github.com/openshift/openshift-velero-plugin":     {"openshift", "openshift-velero-plugin"},
	"github.com/konveyor/openshift-velero-plugin":      {"openshift", "openshift-velero-plugin"},
	"github.com/migtools/udistribution":                {"migtools", "udistribution"},
}

// upstreamToDownstream maps upstream module paths that appear on the LHS
// of replace directives to their downstream fork info. When we see:
//   replace github.com/vmware-tanzu/velero => github.com/openshift/velero v0.10.2-...
// we use this map to identify the downstream fork from the RHS.
var upstreamToDownstream = map[string]struct{ org, repo string }{
	"github.com/vmware-tanzu/velero": {"openshift", "velero"},
	"github.com/kopia/kopia":         {"migtools", "kopia"},
}

// depSyncStore holds DepSync results keyed by "org/repo" so the checker
// can propagate them to RepoStatus after the check runs.
var (
	depSyncStore   = map[string][]DepSync{}
	depSyncStoreMu sync.Mutex

	imageStore   = map[string][]ImageInfo{}
	imageStoreMu sync.Mutex

	konfluxStore   = map[string]*KonfluxInfo{}
	konfluxStoreMu sync.Mutex

	// quayClient is set by main before running checks
	quayClient *QuayClient

	artConfigStore   = map[string][]*ArtBuildConfig{}
	artConfigStoreMu sync.Mutex

	openPRStore   = map[string]*OpenPRInfo{}
	openPRStoreMu sync.Mutex

	// specBranchMap maps "org/repo" to its resolved branch for the current run.
	// Populated once before checks run; read-only during checks (no mutex needed).
	specBranchMap = map[string]string{}
)

// clearStores resets all global check stores between branch runs.
func clearStores() {
	depSyncStoreMu.Lock()
	depSyncStore = map[string][]DepSync{}
	depSyncStoreMu.Unlock()

	imageStoreMu.Lock()
	imageStore = map[string][]ImageInfo{}
	imageStoreMu.Unlock()

	konfluxStoreMu.Lock()
	konfluxStore = map[string]*KonfluxInfo{}
	konfluxStoreMu.Unlock()

	artConfigStoreMu.Lock()
	artConfigStore = map[string][]*ArtBuildConfig{}
	artConfigStoreMu.Unlock()

	openPRStoreMu.Lock()
	openPRStore = map[string]*OpenPRInfo{}
	openPRStoreMu.Unlock()

	gomodDriftStoreMu.Lock()
	gomodDriftStore = map[string][]GoModDrift{}
	gomodDriftStoreMu.Unlock()

	specBranchMap = map[string]string{}
}

// initSpecBranchMap builds the org/repo → branch lookup from all specs.
func initSpecBranchMap(specs []RepoSpec) {
	m := make(map[string]string, len(specs))
	for _, s := range specs {
		m[s.FullName()] = s.Branch
	}
	specBranchMap = m
}

// ---------- Individual checks ----------

// checkConfig verifies a rebase config file exists for this repo/branch.
func checkConfig(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if spec.NoRebase {
		return &CheckResult{StatusNA, "", "not managed by rebasebot"}
	}
	if spec.HasConfig {
		return &CheckResult{StatusOK, "", ""}
	}
	return &CheckResult{
		StatusFail, "",
		fmt.Sprintf("no rebase config for %s/%s on branch %s", spec.Org, spec.Repo, spec.Branch),
	}
}

// checkOpenPR searches for an open rebase PR from oadp-rebasebot on the
// downstream repo. Informational only — presence or absence is not an error.
func checkOpenPR(client *GitHubClient, spec *RepoSpec) *CheckResult {
	pr, err := client.OpenRebasePR(spec.Org, spec.Repo, spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if pr == nil {
		return &CheckResult{StatusNA, "", "no open rebase PR"}
	}

	openPRStoreMu.Lock()
	openPRStore[spec.FullName()] = pr
	openPRStoreMu.Unlock()

	return &CheckResult{StatusOK, fmt.Sprintf("#%d", pr.Number), pr.URL}
}

// checkBranch verifies the downstream branch exists on GitHub.
func checkBranch(client *GitHubClient, spec *RepoSpec) *CheckResult {
	exists, err := client.BranchExists(spec.Org, spec.Repo, spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if exists {
		return &CheckResult{StatusOK, "", ""}
	}
	return &CheckResult{
		StatusFail, "",
		fmt.Sprintf("branch %s not found in %s/%s", spec.Branch, spec.Org, spec.Repo),
	}
}

// checkRebasebotBranch verifies the oadp-rebasebot scratch branch exists.
func checkRebasebotBranch(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if spec.NoRebase {
		return &CheckResult{StatusNA, "", "not managed by rebasebot"}
	}
	if spec.RebasebotRepo == "" {
		// No config → try the default naming convention
		defaultRepo := "oadp-rebasebot/" + spec.Repo
		defaultBranch := "rebase-bot-" + spec.Branch

		exists, err := client.BranchExists("oadp-rebasebot", spec.Repo, defaultBranch)
		if err != nil {
			return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
		}
		if exists {
			return &CheckResult{StatusOK, "", fmt.Sprintf("found %s:%s (inferred)", defaultRepo, defaultBranch)}
		}
		return &CheckResult{
			StatusFail, "",
			fmt.Sprintf("branch %s not found in %s (inferred, no config)", defaultBranch, defaultRepo),
		}
	}

	parts := strings.SplitN(spec.RebasebotRepo, "/", 2)
	if len(parts) != 2 {
		return &CheckResult{StatusWarn, "err", "invalid rebasebot repo format"}
	}

	exists, err := client.BranchExists(parts[0], parts[1], spec.RebasebotBranch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if exists {
		return &CheckResult{StatusOK, "", ""}
	}
	return &CheckResult{
		StatusFail, "",
		fmt.Sprintf("branch %s not found in %s", spec.RebasebotBranch, spec.RebasebotRepo),
	}
}

// checkGoVersion fetches go.mod from the downstream branch and extracts Go version.
func checkGoVersion(client *GitHubClient, spec *RepoSpec) *CheckResult {
	content, err := client.FileContent(spec.Org, spec.Repo, "go.mod", spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if content == nil {
		return &CheckResult{StatusWarn, "N/A", "go.mod not found on branch"}
	}

	gomod := string(content)
	goVer := parseGoVersion(gomod)
	if goVer == "" {
		return &CheckResult{StatusWarn, "?", "could not parse Go version from go.mod"}
	}

	summary := goVer
	if tc := parseGoToolchain(gomod); tc != "" {
		summary += " (tc " + tc + ")"
	}

	return &CheckResult{StatusOK, summary, ""}
}

// checkCIConfig checks if ci-operator config exists in openshift/release.
func checkCIConfig(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if spec.NoRebase {
		return &CheckResult{StatusNA, "", "not managed by rebasebot"}
	}
	if !repoProducesImages(spec.FullName()) {
		return &CheckResult{StatusNA, "", "repo does not produce container images"}
	}
	ciOrg := spec.Org
	ciRepo := spec.Repo

	dirPath := fmt.Sprintf("ci-operator/config/%s/%s", ciOrg, ciRepo)
	files, err := client.DirListing("openshift", "release", dirPath)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if files == nil {
		return &CheckResult{StatusNA, "", "no ci-operator config directory"}
	}

	// Look for a config file matching this branch
	prefix := fmt.Sprintf("%s-%s-%s", ciOrg, ciRepo, spec.Branch)
	for _, f := range files {
		if strings.HasPrefix(f, prefix) && strings.HasSuffix(f, ".yaml") {
			return &CheckResult{StatusOK, "", fmt.Sprintf("found %s", f)}
		}
	}

	return &CheckResult{
		StatusFail, "",
		fmt.Sprintf("no ci-operator config for branch %s in openshift/release", spec.Branch),
	}
}

// checkKonflux checks for Konflux build configuration: .konflux/ directory
// and/or konflux.Dockerfile. When a Dockerfile is found, parses the builder
// image tag from the FROM line (e.g. "rhel_9_golang_1.25").
func checkKonflux(client *GitHubClient, spec *RepoSpec) *CheckResult {
	info := &KonfluxInfo{}

	// Check .konflux/ directory
	dirExists, err := client.DirExists(spec.Org, spec.Repo, ".konflux", spec.Branch)
	if err == nil && dirExists {
		info.HasDir = true
	}

	// Check konflux.Dockerfile
	content, err := client.FileContent(spec.Org, spec.Repo, "konflux.Dockerfile", spec.Branch)
	if err == nil && content != nil {
		info.HasDockerfile = true
		info.BuilderTag = parseKonfluxBuilder(string(content))
	}

	// Store for rendering
	konfluxStoreMu.Lock()
	konfluxStore[spec.FullName()] = info
	konfluxStoreMu.Unlock()

	if !info.HasDir && !info.HasDockerfile {
		return &CheckResult{StatusNA, "", "no .konflux/ or konflux.Dockerfile"}
	}

	summary := info.BuilderTag
	if summary == "" && info.HasDir {
		summary = ".konflux"
	}

	return &CheckResult{StatusOK, summary, ""}
}

// parseKonfluxBuilder extracts the builder image tag from a konflux.Dockerfile.
// Looks for lines like: FROM brew.registry.redhat.io/.../openshift-golang-builder:rhel_9_golang_1.25 AS builder
// Returns just the tag portion (e.g. "rhel_9_golang_1.25").
func parseKonfluxBuilder(dockerfile string) string {
	for _, line := range strings.Split(dockerfile, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Check if this is a builder stage (has "AS builder" suffix)
		isBuilder := false
		for i, f := range fields {
			if strings.EqualFold(f, "AS") && i+1 < len(fields) && strings.EqualFold(fields[i+1], "builder") {
				isBuilder = true
				break
			}
		}
		if !isBuilder {
			continue
		}
		image := fields[1]
		if idx := strings.LastIndex(image, ":"); idx != -1 {
			return image[idx+1:]
		}
	}
	return ""
}

// checkUpstreamImage checks Quay.io for the existence and age of container images
// tagged with the repo's branch name. Merges images from image-references and the
// hardcoded repoImages map so all known images for a repo are checked and visible.
func checkUpstreamImage(client *GitHubClient, spec *RepoSpec) *CheckResult {
	// Collect images from both sources, deduplicating by quay repo name
	seen := map[string]bool{}
	var quayChecks []QuayImage

	// Source 1: image-references (data-driven)
	if currentReleaseData != nil {
		if refs := currentReleaseData.ImageRefsFor(spec.FullName()); len(refs) > 0 {
			for _, ref := range refs {
				if ref.Namespace != "" && ref.QuayRepo != "" && !seen[ref.QuayRepo] {
					seen[ref.QuayRepo] = true
					quayChecks = append(quayChecks, QuayImage{
						Namespace: ref.Namespace,
						Repo:     ref.QuayRepo,
						Name:     ref.QuayRepo,
					})
				}
			}
		}
	}

	// Source 2: hardcoded repoImages (fills gaps not covered by image-references)
	if images, ok := repoImages[spec.FullName()]; ok {
		for _, img := range images {
			if !seen[img.Repo] {
				seen[img.Repo] = true
				quayChecks = append(quayChecks, img)
			}
		}
	}

	if len(quayChecks) == 0 {
		return &CheckResult{StatusNA, "", "no images defined"}
	}

	if quayClient == nil {
		return &CheckResult{StatusWarn, "err", "quay client not initialized"}
	}

	tag := imageTag(spec.Branch)
	var infos []ImageInfo
	missing := 0

	for _, img := range quayChecks {
		info := ImageInfo{
			Name:      img.Name,
			Namespace: img.Namespace,
			Repo:      img.Repo,
			Tag:       tag,
		}

		tagInfo, err := quayClient.TagInfo(img.Namespace, img.Repo, tag)
		if err != nil {
			info.Exists = false
		} else if tagInfo.Exists {
			info.Exists = true
			info.LastModified = tagInfo.LastModified
		}

		if !info.Exists {
			missing++
		}
		infos = append(infos, info)
	}

	// Store for rendering
	imageStoreMu.Lock()
	imageStore[spec.FullName()] = infos
	imageStoreMu.Unlock()

	if missing == len(infos) {
		return &CheckResult{StatusFail, fmt.Sprintf("0/%d", len(infos)),
			fmt.Sprintf("no images found with tag %s on quay.io; push upstream images", tag)}
	}
	if missing > 0 {
		return &CheckResult{StatusWarn, fmt.Sprintf("%d/%d", len(infos)-missing, len(infos)),
			fmt.Sprintf("%d image(s) missing tag %s on quay.io", missing, tag)}
	}

	// All present — show age of oldest
	oldest := infos[0].LastModified
	for _, info := range infos[1:] {
		if info.LastModified.Before(oldest) {
			oldest = info.LastModified
		}
	}
	return &CheckResult{StatusOK, FormatAge(oldest), ""}
}

// checkArtConfig checks if ocp-build-data has build config(s) for this repo.
// Repos that produce multiple images (e.g. oadp-vm-file-restore) need multiple configs.
func checkArtConfig(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if currentReleaseData == nil || !currentReleaseData.HasArtBranch {
		return &CheckResult{StatusNA, "", "no ocp-build-data branch for this release"}
	}

	key := spec.FullName()
	cfgs := currentReleaseData.ArtConfigsFor(key)
	if len(cfgs) == 0 {
		if !repoProducesImages(key) {
			return &CheckResult{StatusNA, "", "repo does not produce container images"}
		}
		return &CheckResult{
			StatusFail, "",
			fmt.Sprintf("no ocp-build-data config; add YAML to https://github.com/openshift-eng/ocp-build-data/tree/%s/images",
				spec.Branch),
		}
	}

	// Store for rendering
	artConfigStoreMu.Lock()
	artConfigStore[key] = cfgs
	artConfigStoreMu.Unlock()

	// Check for disabled configs
	disabled := 0
	for _, cfg := range cfgs {
		if cfg.Disabled() {
			disabled++
		}
	}
	if disabled > 0 {
		detail := fmt.Sprintf("%d of %d ART config(s) disabled; enable in https://github.com/openshift-eng/ocp-build-data/tree/%s/images",
			disabled, len(cfgs), spec.Branch)
		if disabled == len(cfgs) {
			return &CheckResult{StatusFail, "disabled", detail}
		}
		return &CheckResult{StatusWarn, fmt.Sprintf("%d/%d disabled", disabled, len(cfgs)), detail}
	}

	// Check if we have enough configs for the expected image count
	expectedCount := expectedImageCount(key)
	if expectedCount > 0 && len(cfgs) < expectedCount {
		return &CheckResult{
			StatusWarn,
			fmt.Sprintf("%d/%d", len(cfgs), expectedCount),
			fmt.Sprintf("only %d of %d expected ocp-build-data configs; add missing YAML to https://github.com/openshift-eng/ocp-build-data/tree/%s/images",
				len(cfgs), expectedCount, spec.Branch),
		}
	}

	// Cross-reference: verify image-references names match ocp-build-data names
	if currentReleaseData != nil && currentReleaseData.HasImageRefs {
		if mismatches := crossRefImageArt(key, cfgs, currentReleaseData.ImageRefsFor(key), spec.Branch); mismatches != "" {
			summary := ""
			if len(cfgs) == 1 && cfgs[0].Dockerfile != "" {
				summary = cfgs[0].Dockerfile
			} else if len(cfgs) > 1 {
				summary = fmt.Sprintf("%d cfgs", len(cfgs))
			}
			return &CheckResult{StatusWarn, summary, mismatches}
		}
	}

	summary := ""
	if len(cfgs) == 1 && cfgs[0].Dockerfile != "" {
		summary = cfgs[0].Dockerfile
	} else if len(cfgs) > 1 {
		summary = fmt.Sprintf("%d cfgs", len(cfgs))
	}

	return &CheckResult{StatusOK, summary, ""}
}

// crossRefImageArt verifies that image-references names match ocp-build-data names.
// Each image-references entry's ARTName should have a matching ART config, and vice versa.
// Returns an error message describing mismatches, or "" if everything is in sync.
func crossRefImageArt(orgRepo string, cfgs []*ArtBuildConfig, refs []*ImageRefEntry, branch string) string {
	if len(refs) == 0 || len(cfgs) == 0 {
		return ""
	}

	// Build lookup sets
	artNames := make(map[string]bool, len(cfgs))
	for _, cfg := range cfgs {
		artNames[cfg.ARTName()] = true
	}

	refNames := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if ref.ARTName != "" {
			refNames[ref.ARTName] = true
		}
	}

	// Check for image-references entries without matching ART config
	var missingInArt []string
	for name := range refNames {
		if !artNames[name] {
			missingInArt = append(missingInArt, name)
		}
	}

	// Check for ART configs without matching image-references entry
	var missingInRefs []string
	for name := range artNames {
		if !refNames[name] {
			missingInRefs = append(missingInRefs, name)
		}
	}

	if len(missingInArt) == 0 && len(missingInRefs) == 0 {
		return ""
	}

	var parts []string
	if len(missingInArt) > 0 {
		parts = append(parts, fmt.Sprintf(
			"image-references name(s) %v not found in ocp-build-data/%s/images",
			missingInArt, branch))
	}
	if len(missingInRefs) > 0 {
		parts = append(parts, fmt.Sprintf(
			"ocp-build-data name(s) %v not found in image-references",
			missingInRefs))
	}
	return strings.Join(parts, "; ")
}

// expectedImageCount returns the number of images a repo is expected to produce,
// based on image-references entries and the hardcoded repoImages map.
func expectedImageCount(orgRepo string) int {
	// Prefer image-references count (data-driven)
	if currentReleaseData != nil {
		if refs := currentReleaseData.ImageRefsFor(orgRepo); len(refs) > 0 {
			return len(refs)
		}
	}
	// Fall back to hardcoded map
	if images, ok := repoImages[orgRepo]; ok {
		return len(images)
	}
	return 0
}

// checkProductized checks if the repo's image is active in bundle/image-references.
func checkProductized(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if currentReleaseData == nil || !currentReleaseData.HasImageRefs {
		return &CheckResult{StatusNA, "", "no bundle/image-references for this release"}
	}

	key := spec.FullName()
	refs := currentReleaseData.ImageRefsFor(key)
	if len(refs) == 0 {
		if !repoProducesImages(key) {
			return &CheckResult{StatusNA, "", "repo does not produce container images"}
		}
		return &CheckResult{
			StatusWarn, "",
			fmt.Sprintf("not in image-references; add entry to https://github.com/openshift/oadp-operator/blob/%s/bundle/image-references",
				spec.Branch),
		}
	}

	// Count active vs commented entries
	active := 0
	for _, ref := range refs {
		if !ref.CommentedOut {
			active++
		}
	}

	if active == 0 {
		return &CheckResult{
			StatusWarn,
			fmt.Sprintf("0/%d", len(refs)),
			fmt.Sprintf("all %d entries commented out in image-references; uncomment in https://github.com/openshift/oadp-operator/blob/%s/bundle/image-references when ready",
				len(refs), spec.Branch),
		}
	}

	if active < len(refs) {
		return &CheckResult{
			StatusWarn,
			fmt.Sprintf("%d/%d", active, len(refs)),
			fmt.Sprintf("%d of %d entries still commented out", len(refs)-active, len(refs)),
		}
	}

	return &CheckResult{StatusOK, fmt.Sprintf("%d/%d", active, len(refs)), ""}
}

// checkDepSync verifies that go.mod references to internal OADP dependencies
// point to the HEAD commit of the dependency's branch. If a repo's go.mod
// uses a pseudo-version referencing an older commit, it is flagged as out-of-sync.
func checkDepSync(client *GitHubClient, spec *RepoSpec) *CheckResult {
	content, err := client.FileContent(spec.Org, spec.Repo, "go.mod", spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}

	var syncs []DepSync
	if content != nil {
		gomod := string(content)
		syncs = parseInternalDeps(gomod, spec.Branch, spec.Org+"/"+spec.Repo)
	}

	// Check git submodules (.gitmodules + tree entries with type "commit")
	gitmodulesContent, err := client.FileContent(spec.Org, spec.Repo, ".gitmodules", spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error loading .gitmodules: %v", err)}
	}
	if gitmodulesContent != nil {
		treeEntries, err := client.SubmoduleEntries(spec.Org, spec.Repo, spec.Branch)
		if err != nil {
			return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error loading submodule tree: %v", err)}
		}
		subSyncs := parseSubmoduleDeps(string(gitmodulesContent), treeEntries, spec.Org+"/"+spec.Repo)
		syncs = append(syncs, subSyncs...)
	}

	if len(syncs) == 0 {
		return &CheckResult{StatusOK, "", "no internal deps"}
	}

	// Resolve HEAD commits for each dependency
	outOfSync := 0
	unresolved := 0
	for i := range syncs {
		dep := &syncs[i]
		depKey := dep.Org + "/" + dep.Repo
		depBranch := spec.Branch
		if b, ok := specBranchMap[depKey]; ok {
			depBranch = b
		}
		if dep.SubmoduleBranch != "" {
			depBranch = dep.SubmoduleBranch
		}
		head, err := client.HeadCommitSHA(dep.Org, dep.Repo, depBranch)
		if err != nil || head == "" {
			unresolved++
			continue
		}
		dep.HeadHash = head
		dep.InSync = strings.HasPrefix(head, dep.HaveHash)
		if !dep.InSync {
			outOfSync++
		}
	}

	// Store dep syncs for later propagation
	depSyncStoreMu.Lock()
	depSyncStore[spec.FullName()] = syncs
	depSyncStoreMu.Unlock()

	if unresolved > 0 {
		return &CheckResult{
			StatusWarn,
			fmt.Sprintf("%d/%d", len(syncs)-outOfSync-unresolved, len(syncs)),
			fmt.Sprintf("could not resolve %d internal dep(s)", unresolved),
		}
	}
	if outOfSync > 0 {
		return &CheckResult{
			StatusFail,
			fmt.Sprintf("%d/%d", len(syncs)-outOfSync, len(syncs)),
			fmt.Sprintf("%d internal dep(s) out of sync", outOfSync),
		}
	}
	return &CheckResult{StatusOK, fmt.Sprintf("%d/%d", len(syncs), len(syncs)), ""}
}

// parseInternalDeps scans go.mod for references to known downstream modules
// and extracts pseudo-version commit hashes. It handles three patterns:
//
//  1. Replace with upstream LHS:
//     replace github.com/vmware-tanzu/velero => github.com/openshift/velero v0.10.2-0.20250313...-584cf1148a74
//
//  2. Replace with downstream LHS (self-referencing or renaming):
//     replace github.com/kopia/kopia => github.com/migtools/kopia v0.0.0-20260211...-b68c22afd36d
//
//  3. Direct require with pseudo-version:
//     github.com/openshift/oadp-operator v1.0.2-0.20260202155540-e1dcfd104852
func parseInternalDeps(gomod, branch, selfRepo string) []DepSync {
	seen := map[string]bool{} // track org/repo to avoid duplicates
	var syncs []DepSync

	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(line)

		// Pattern 1 & 2: replace directives
		// replace <upstream> => <downstream> <pseudo-version>
		if strings.HasPrefix(line, "replace ") {
			parts := strings.Fields(line)
			// replace <mod> => <target> <version>
			arrowIdx := -1
			for i, p := range parts {
				if p == "=>" {
					arrowIdx = i
					break
				}
			}
			if arrowIdx < 0 || arrowIdx+2 >= len(parts) {
				continue
			}
			targetMod := parts[arrowIdx+1]
			targetVer := parts[arrowIdx+2]

			hash := extractPseudoHash(targetVer)
			if hash == "" {
				continue
			}

			// Check if target is a known downstream module
			if info, ok := downstreamModules[targetMod]; ok {
				key := info.org + "/" + info.repo
				if key != selfRepo && !seen[key] {
					seen[key] = true
					syncs = append(syncs, DepSync{
						Module:   targetMod,
						Org:      info.org,
						Repo:     info.repo,
						HaveHash: hash,
					})
				}
				continue
			}

			// Check if the LHS upstream module maps to a downstream
			lhsMod := parts[1]
			if info, ok := upstreamToDownstream[lhsMod]; ok {
				key := info.org + "/" + info.repo
				if key != selfRepo && !seen[key] {
					seen[key] = true
					syncs = append(syncs, DepSync{
						Module:   targetMod,
						Org:      info.org,
						Repo:     info.repo,
						HaveHash: hash,
					})
				}
			}
			continue
		}

		// Pattern 3: direct require lines
		// github.com/openshift/oadp-operator v1.0.2-0.20260202155540-e1dcfd104852
		for modPath, info := range downstreamModules {
			if !strings.Contains(line, modPath) {
				continue
			}
			// Skip "replace" lines (already handled above) and comments
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "replace") {
				continue
			}

			fields := strings.Fields(line)
			for fi, f := range fields {
				if f == modPath && fi+1 < len(fields) {
					hash := extractPseudoHash(fields[fi+1])
					if hash == "" {
						continue
					}
					key := info.org + "/" + info.repo
					if key != selfRepo && !seen[key] {
						seen[key] = true
						syncs = append(syncs, DepSync{
							Module:   modPath,
							Org:      info.org,
							Repo:     info.repo,
							HaveHash: hash,
						})
					}
				}
			}
		}
	}

	return syncs
}

// extractPseudoHash returns the 12-char commit hash from a Go pseudo-version,
// or "" if the version string is not a pseudo-version.
// e.g. "v0.10.2-0.20250313160323-584cf1148a74" -> "584cf1148a74"
func extractPseudoHash(version string) string {
	if pseudoHashRe.MatchString(version) {
		return version[len(version)-12:]
	}
	return ""
}

// Matches Go pseudo-versions including pre-release tags:
//   v0.0.0-20250313160323-584cf1148a74                    (base version)
//   v0.10.2-0.20250313160323-584cf1148a74                 (pre-release)
//   v0.13.0-velero.1.0.20260706195151-83febd3dd228        (tagged pre-release)
var pseudoHashRe = regexp.MustCompile(`v\d+\.\d+\.\d+-([a-zA-Z0-9]+\.)*\d{14}-[0-9a-f]{12}$`)

// ---------- Parsing helpers ----------

var goVersionRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)`)
var goToolchainRe = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+(?:\.\d+)?)`)

func parseGoVersion(gomod string) string {
	m := goVersionRe.FindStringSubmatch(gomod)
	if m == nil {
		return ""
	}
	return m[1]
}

func parseGoToolchain(gomod string) string {
	m := goToolchainRe.FindStringSubmatch(gomod)
	if m == nil {
		return ""
	}
	return m[1]
}

func parseUpstreamFromGoMod(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "replace github.com/vmware-tanzu/velero") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}
