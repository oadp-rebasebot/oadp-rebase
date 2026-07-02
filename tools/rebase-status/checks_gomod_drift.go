package main

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/mod/semver"
)

var (
	gomodDriftStore   = map[string][]GoModDrift{}
	gomodDriftStoreMu sync.Mutex
)

// checkGoModDrift compares direct dependency versions between the downstream
// go.mod and the upstream source go.mod. Reports downgrades as warnings.
func checkGoModDrift(client *GitHubClient, spec *RepoSpec) *CheckResult {
	if spec.Upstream == "" {
		return &CheckResult{StatusNA, "", "no upstream source configured"}
	}

	upOrg, upRepo, upBranch, err := parseRepoRef(spec.Upstream)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("invalid upstream ref: %v", err)}
	}
	if upBranch == "" {
		return &CheckResult{StatusWarn, "err", "upstream ref has no branch"}
	}

	downBytes, err := client.FileContent(spec.Org, spec.Repo, "go.mod", spec.Branch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("fetch downstream go.mod: %v", err)}
	}
	if downBytes == nil {
		return &CheckResult{StatusNA, "", "no go.mod in downstream"}
	}

	upBytes, err := client.FileContent(upOrg, upRepo, "go.mod", upBranch)
	if err != nil {
		return &CheckResult{StatusWarn, "err", fmt.Sprintf("fetch upstream go.mod: %v", err)}
	}
	if upBytes == nil {
		return &CheckResult{StatusNA, "", "no go.mod in upstream"}
	}

	downGoMod := string(downBytes)
	upGoMod := string(upBytes)

	drifts := compareGoModVersions(
		parseGoModRequires(downGoMod),
		parseGoModReplaces(downGoMod),
		parseGoModRequires(upGoMod),
		parseGoModReplaces(upGoMod),
	)

	gomodDriftStoreMu.Lock()
	gomodDriftStore[spec.FullName()] = drifts
	gomodDriftStoreMu.Unlock()

	if len(drifts) == 0 {
		return &CheckResult{StatusOK, "", "no dependency downgrades vs upstream"}
	}

	return &CheckResult{
		StatusWarn,
		fmt.Sprintf("%d", len(drifts)),
		fmt.Sprintf("%d direct deps downgraded vs upstream", len(drifts)),
	}
}

// parseGoModRequires extracts direct dependencies from go.mod require blocks.
// Returns module path → version for lines without "// indirect".
func parseGoModRequires(gomod string) map[string]string {
	result := make(map[string]string)
	inBlock := false

	for _, line := range strings.Split(gomod, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "require (") || trimmed == "require (" {
			inBlock = true
			continue
		}
		if inBlock && trimmed == ")" {
			inBlock = false
			continue
		}

		if strings.Contains(trimmed, "// indirect") {
			continue
		}
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}

		if inBlock {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				result[fields[0]] = fields[1]
			}
		} else if strings.HasPrefix(trimmed, "require ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 {
				result[fields[1]] = fields[2]
			}
		}
	}

	return result
}

// replaceEntry holds a parsed replace directive.
type replaceEntry struct {
	ReplacementModule string // RHS module path
	Version           string // RHS version
}

// parseGoModReplaces extracts replace directives from go.mod.
// Returns the original module path → replacement info.
// Skips local-path replaces (no version on the RHS).
func parseGoModReplaces(gomod string) map[string]replaceEntry {
	result := make(map[string]replaceEntry)
	inBlock := false

	for _, line := range strings.Split(gomod, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "replace (") || trimmed == "replace (" {
			inBlock = true
			continue
		}
		if inBlock && trimmed == ")" {
			inBlock = false
			continue
		}
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}

		var directive string
		if inBlock {
			directive = trimmed
		} else if strings.HasPrefix(trimmed, "replace ") {
			directive = strings.TrimPrefix(trimmed, "replace ")
		} else {
			continue
		}

		parts := strings.SplitN(directive, "=>", 2)
		if len(parts) != 2 {
			continue
		}

		lhsFields := strings.Fields(strings.TrimSpace(parts[0]))
		rhsFields := strings.Fields(strings.TrimSpace(parts[1]))

		if len(lhsFields) < 1 || len(rhsFields) < 2 {
			continue // no version on RHS (local path replace)
		}

		module := lhsFields[0]
		result[module] = replaceEntry{
			ReplacementModule: rhsFields[0],
			Version:           rhsFields[len(rhsFields)-1],
		}
	}

	return result
}

// compareGoModVersions finds direct dependencies where downstream has an older
// version than upstream. Replaces override requires for effective version.
// Skips modules where downstream replaces to a different repo than upstream
// (fork-based replaces make version comparison meaningless).
func compareGoModVersions(
	downReqs map[string]string, downReplaces map[string]replaceEntry,
	upReqs map[string]string, upReplaces map[string]replaceEntry,
) []GoModDrift {
	var drifts []GoModDrift

	for module, upVersion := range upReqs {
		downVersion, inDown := downReqs[module]
		if !inDown {
			continue
		}

		upEffective := upVersion
		downEffective := downVersion

		upRepl, upHasReplace := upReplaces[module]
		downRepl, downHasReplace := downReplaces[module]

		if upHasReplace {
			upEffective = upRepl.Version
		}
		if downHasReplace {
			downEffective = downRepl.Version
		}

		// Skip when replaces point to different repos — the downstream
		// is using a fork, so version comparison is meaningless.
		// Cases: downstream has replace but upstream doesn't, or they
		// replace to different modules.
		if downHasReplace && !upHasReplace {
			if downRepl.ReplacementModule != module {
				continue
			}
		}
		if downHasReplace && upHasReplace {
			if downRepl.ReplacementModule != upRepl.ReplacementModule {
				continue
			}
		}

		if !semver.IsValid(downEffective) || !semver.IsValid(upEffective) {
			continue
		}

		if semver.Compare(downEffective, upEffective) < 0 {
			drifts = append(drifts, GoModDrift{
				Module:            module,
				DownstreamVersion: downEffective,
				UpstreamVersion:   upEffective,
			})
		}
	}

	return drifts
}
