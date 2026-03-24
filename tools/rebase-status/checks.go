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
	{ID: "config", Header: "Rebase Cfg", Run: checkConfig},
	{ID: "rebasebot", Header: "Rebase", Run: checkRebasebotBranch},
	{ID: "go_version", Header: "Go", Run: checkGoVersion},
	{ID: "ci_config", Header: "Prow Cfg", Run: checkCIConfig},
	{ID: "dep_sync", Header: "Deps", Run: checkDepSync},
	{ID: "konflux", Header: "Konflux", Run: checkKonflux},
	{ID: "image_sync", Header: "Image", Run: checkImageSync},
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
)

// ---------- Individual checks ----------

// checkConfig verifies a rebase config file exists for this repo/branch.
func checkConfig(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if spec.HasConfig {
		return &CheckResult{StatusOK, "", ""}
	}
	return &CheckResult{
		StatusFail, "",
		fmt.Sprintf("no rebase config for %s/%s on branch %s", spec.Org, spec.Repo, spec.Branch),
	}
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

	goVer := parseGoVersion(string(content))
	if goVer == "" {
		return &CheckResult{StatusWarn, "?", "could not parse Go version from go.mod"}
	}

	return &CheckResult{StatusOK, goVer, ""}
}

// checkCIConfig checks if ci-operator config exists in openshift/release.
func checkCIConfig(client *GitHubClient, spec *RepoSpec) *CheckResult {
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

// checkImageSync checks Quay.io for the existence and age of container images
// tagged with the repo's branch name.
func checkImageSync(client *GitHubClient, spec *RepoSpec) *CheckResult {
	images, ok := repoImages[spec.FullName()]
	if !ok || len(images) == 0 {
		return &CheckResult{StatusNA, "", "no images defined"}
	}

	if quayClient == nil {
		return &CheckResult{StatusWarn, "err", "quay client not initialized"}
	}

	tag := spec.Branch
	// oadp-dev and main branches publish to :latest tag
	if tag == "oadp-dev" || tag == "main" {
		tag = "latest"
	}
	var infos []ImageInfo
	missing := 0

	for _, img := range images {
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
			fmt.Sprintf("no images found with tag %s", tag)}
	}
	if missing > 0 {
		return &CheckResult{StatusWarn, fmt.Sprintf("%d/%d", len(infos)-missing, len(infos)),
			fmt.Sprintf("%d image(s) missing tag %s", missing, tag)}
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

// checkDepSync verifies that go.mod references to internal OADP dependencies
// point to the HEAD commit of the dependency's branch. If a repo's go.mod
// uses a pseudo-version referencing an older commit, it is flagged as out-of-sync.
func checkDepSync(client *GitHubClient, spec *RepoSpec) *CheckResult {
	content, err := client.FileContent(spec.Org, spec.Repo, "go.mod", spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("API error: %v", err)}
	}
	if content == nil {
		return &CheckResult{StatusNA, "", "no go.mod"}
	}

	gomod := string(content)
	syncs := parseInternalDeps(gomod, spec.Branch, spec.Org+"/"+spec.Repo)

	if len(syncs) == 0 {
		return &CheckResult{StatusOK, "", "no internal deps"}
	}

	// Resolve HEAD commits for each dependency
	outOfSync := 0
	for i := range syncs {
		dep := &syncs[i]
		head, err := client.HeadCommitSHA(dep.Org, dep.Repo, spec.Branch)
		if err != nil || head == "" {
			continue // skip if we can't resolve
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

// Matches Go pseudo-versions in all three forms:
//   v0.0.0-20250313160323-584cf1148a74          (base version)
//   v0.10.2-0.20250313160323-584cf1148a74       (pre-release, e.g. after a tag)
//   v1.0.2-0.20260202155540-e1dcfd104852        (pre-release)
var pseudoHashRe = regexp.MustCompile(`v\d+\.\d+\.\d+-(0\.)?\d{14}-[0-9a-f]{12}$`)

// ---------- Parsing helpers ----------

var goVersionRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)`)

func parseGoVersion(gomod string) string {
	m := goVersionRe.FindStringSubmatch(gomod)
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
