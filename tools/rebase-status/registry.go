package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Wave metadata — populated from repos.yaml at startup.
var wavesMeta []WaveInfo

type RepoDef struct {
	Org       string
	Repo      string
	Wave      int
	MainOnly  bool
	NoRebase  bool
	Branch    string
	MinBranch string
	MaxBranch string // latest release branch (e.g. "oadp-1.3"); empty = all branches
	DevBranch string // branch to use when target is oadp-dev (e.g. "main"); empty = use oadp-dev
}

// allRepos is populated from repos.yaml at startup.
var allRepos []RepoDef

// DisplayGroup defines a visual column in the output that combines
// one or more individual check results. Individual checks still run
// independently and generate granular issues; display groups only
// control the table layout.
type DisplayGroup struct {
	ID       string   // column identifier
	Header   string   // column header text
	CheckIDs []string // which check IDs contribute to this column
}

// DisplayGroups defines the consolidated columns shown in table/text/markdown output.
var DisplayGroups = []DisplayGroup{
	{ID: "open_pr", Header: "PR", CheckIDs: []string{"open_pr"}},
	{ID: "rebase", Header: "Rebase", CheckIDs: []string{"config", "rebasebot"}},
	{ID: "upstream_sync", Header: "Upstream", CheckIDs: []string{"upstream_sync"}},
	{ID: "go_version", Header: "Go", CheckIDs: []string{"go_version"}},
	{ID: "ci_config", Header: "CI", CheckIDs: []string{"ci_config"}},
	{ID: "dep_sync", Header: "Deps", CheckIDs: []string{"dep_sync"}},
	{ID: "gomod_drift", Header: "Drift", CheckIDs: []string{"gomod_drift"}},
	{ID: "quay", Header: "Quay", CheckIDs: []string{"upstream_image", "productized"}},
	{ID: "konflux", Header: "Konflux", CheckIDs: []string{"konflux", "art_config"}},
}

// QuayImage describes a container image on Quay.io.
type QuayImage struct {
	Namespace string // e.g. "konveyor"
	Repo      string // e.g. "velero"
	Name      string // display name, e.g. "velero"
}

// repoImages is populated from repos.yaml at startup.
var repoImages map[string][]QuayImage

// filenameToRepo is populated from repos.yaml at startup.
var filenameToRepo map[string]struct{ org, repo string }

// RebaseConfigFilename returns the config filename for a given org/repo/branch.
// e.g. ("openshift", "velero", "oadp-1.6") → "openshift_velero_oadp-1.6.env.sh"
func RebaseConfigFilename(org, repo, branch string) string {
	for prefix, entry := range filenameToRepo {
		if entry.org == org && entry.repo == repo {
			return prefix + "_" + branch + ".env.sh"
		}
	}
	return ""
}

// LoadSpecs builds a RepoSpec list for the given branch.
// It starts from the canonical allRepos catalog, then enriches with
// config file data when available. Repos without configs are still
// included (with a "missing config" flag) so the report is complete.
func LoadSpecs(configDir, branch string) ([]RepoSpec, error) {
	// Load all available config files into a map keyed by "org/repo"
	configs := loadConfigs(configDir, branch)

	var specs []RepoSpec
	for _, def := range allRepos {
		if def.MainOnly && branch != "oadp-dev" {
			continue
		}

		if def.MinBranch != "" && !branchAtLeast(branch, def.MinBranch) {
			continue
		}

		if def.MaxBranch != "" && !branchAtMost(branch, def.MaxBranch) {
			continue
		}

		key := def.Org + "/" + def.Repo
		spec := RepoSpec{
			Org:    def.Org,
			Repo:   def.Repo,
			Branch: branch,
			Wave:   def.Wave,
		}

		if def.MainOnly {
			spec.Branch = "main"
		}

		if def.DevBranch != "" && branch == "oadp-dev" {
			spec.Branch = def.DevBranch
		}

		// NoRebase repos: set flag and use explicit branch (default "main")
		if def.NoRebase {
			spec.NoRebase = true
			if def.Branch != "" {
				spec.Branch = def.Branch
			} else {
				spec.Branch = "main"
			}
		}

		// Enrich with config file data if available
		if cfg, ok := configs[key]; ok {
			spec.HasConfig = true
			spec.Skip = cfg.Skip
			spec.Upstream = cfg.Upstream
			spec.RebasebotRepo = cfg.RebasebotRepo
			spec.RebasebotBranch = cfg.RebasebotBranch
			// Config may override the branch (e.g. "main" for main-tracking repos)
			if cfg.Branch != "" {
				spec.Branch = cfg.Branch
			}
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// configData holds parsed data from a config file.
type configData struct {
	Branch          string
	Skip            bool
	Upstream        string
	RebasebotRepo   string
	RebasebotBranch string
}

// loadConfigs parses all config files for a branch and returns a map
// keyed by "org/repo".
func loadConfigs(configDir, branch string) map[string]*configData {
	result := make(map[string]*configData)

	patterns := []string{
		filepath.Join(configDir, fmt.Sprintf("*_%s.env.sh", branch)),
	}
	// Include main-tracking configs when checking oadp-dev
	if branch == "oadp-dev" {
		patterns = append(patterns, filepath.Join(configDir, "*_main.env.sh"))
	}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, f := range files {
			cfg, org, repo := parseConfigFileToData(f, branch)
			if cfg != nil && org != "" {
				result[org+"/"+repo] = cfg
			}
		}
	}

	return result
}

// parseConfigFileToData reads a config file and returns parsed data
// plus the org/repo it belongs to.
func parseConfigFileToData(path, targetBranch string) (*configData, string, string) {
	vars, err := parseShellVars(path)
	if err != nil {
		return nil, "", ""
	}

	// Load SSOT versions file and merge (config vars take precedence)
	versionsDir := filepath.Join(filepath.Dir(path), "..", "versions")
	versionsFile := filepath.Join(versionsDir, targetBranch+".env")
	if versionVars, verr := parseShellVars(versionsFile); verr == nil {
		for k, v := range versionVars {
			if _, exists := vars[k]; !exists {
				vars[k] = v
			}
		}
	}

	expandVars(vars)

	cfg := &configData{
		Skip: vars["SKIP_REPO"] == "true",
	}

	dest := vars["DESTINATION_DOWNSTREAM_REPO"]
	var org, repo string

	if dest != "" {
		var branch string
		org, repo, branch, err = parseRepoRef(dest)
		if err != nil {
			// Try to derive from filename
			org, repo, _ = orgRepoFromFilename(path, targetBranch)
		} else {
			cfg.Branch = branch
		}
	} else {
		// Derive from filename (e.g. SKIP-only configs)
		org, repo, _ = orgRepoFromFilename(path, targetBranch)
	}

	if org == "" || repo == "" {
		return nil, "", ""
	}

	// Upstream
	source := vars["SOURCE_UPSTREAM_REPO"]
	if source != "" {
		cfg.Upstream = source
		sourceOrg, sourceRepo, _, _ := parseRepoRef(strings.TrimPrefix(source, "https://github.com/"))
		if sourceOrg == org && sourceRepo == repo {
			cfg.Upstream = "" // downstream-only
		}
	}

	// Rebasebot
	rebase := vars["REBASE_REPO"]
	if rebase != "" {
		parts := strings.SplitN(rebase, ":", 2)
		cfg.RebasebotRepo = parts[0]
		if len(parts) > 1 {
			cfg.RebasebotBranch = parts[1]
		}
	}

	return cfg, org, repo
}

// LoadVersionsVars loads the SSOT versions file for a branch and returns
// the parsed key-value pairs. Returns nil, nil if the file does not exist.
func LoadVersionsVars(configDir, branch string) (map[string]string, error) {
	versionsDir := filepath.Join(configDir, "..", "versions")
	versionsFile := filepath.Join(versionsDir, branch+".env")

	if _, err := os.Stat(versionsFile); os.IsNotExist(err) {
		return nil, nil
	}

	vars, err := parseShellVars(versionsFile)
	if err != nil {
		return nil, err
	}
	expandVars(vars)
	return vars, nil
}

// parseShellVars extracts VAR="value" and VAR=value assignments.
func parseShellVars(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	reQuoted := regexp.MustCompile(`^([A-Z_][A-Z_0-9]*)="([^"]*)"`)
	reUnquoted := regexp.MustCompile(`^([A-Z_][A-Z_0-9]*)=([^\s"#\\]+)$`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			vars[m[1]] = m[2]
		} else if m := reUnquoted.FindStringSubmatch(line); m != nil {
			vars[m[1]] = m[2]
		}
	}
	return vars, scanner.Err()
}

// expandVars resolves $VAR, ${VAR}, and ${VAR:?error} references within values.
func expandVars(vars map[string]string) {
	varRefRe := regexp.MustCompile(`\$\{([A-Z_][A-Z_0-9]*)(?::[\?+-][^}]*)?\}|\$([A-Z_][A-Z_0-9]*)`)

	for pass := 0; pass < 3; pass++ {
		changed := false
		for k, v := range vars {
			expanded := varRefRe.ReplaceAllStringFunc(v, func(match string) string {
				sub := varRefRe.FindStringSubmatch(match)
				name := sub[1]
				if name == "" {
					name = sub[2]
				}
				if val, ok := vars[name]; ok {
					changed = true
					return val
				}
				return match
			})
			vars[k] = expanded
		}
		if !changed {
			break
		}
	}
}

// orgRepoFromFilename derives org and repo from a config filename.
func orgRepoFromFilename(path, branch string) (org, repo string, ok bool) {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".env.sh")

	branchSuffix := "_" + branch
	if !strings.HasSuffix(base, branchSuffix) {
		branchSuffix = "_main"
		if !strings.HasSuffix(base, branchSuffix) {
			return "", "", false
		}
	}
	prefix := strings.TrimSuffix(base, branchSuffix)

	if entry, found := filenameToRepo[prefix]; found {
		return entry.org, entry.repo, true
	}
	return "", "", false
}

// parseRepoRef splits "org/repo:branch" into components.
func parseRepoRef(ref string) (org, repo, branch string, err error) {
	ref = strings.TrimPrefix(ref, "https://github.com/")
	parts := strings.SplitN(ref, ":", 2)
	orgRepo := parts[0]
	if len(parts) > 1 {
		branch = parts[1]
	}
	slash := strings.SplitN(orgRepo, "/", 2)
	if len(slash) != 2 {
		return "", "", "", fmt.Errorf("expected org/repo, got %q", orgRepo)
	}
	return slash[0], slash[1], branch, nil
}

// FindConfigDir walks up from the current directory looking for rebase-configs/.
func FindConfigDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "rebase-configs")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("rebase-configs/ not found (searched from cwd upward)")
}

// parseBranchVersion extracts (major, minor) from an "oadp-X.Y" branch name.
// Returns (-1, -1) for non-release branches like "oadp-dev" or "main".
func parseBranchVersion(branch string) (int, int) {
	rest := strings.TrimPrefix(branch, "oadp-")
	if rest == branch {
		return -1, -1 // not an oadp- branch
	}
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 {
		return -1, -1
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return -1, -1
	}
	return major, minor
}

// branchAtMost returns true if branch <= maxBranch in version ordering.
// Non-release branches (oadp-dev, main) always return true (they track tip).
func branchAtMost(branch, maxBranch string) bool {
	maj, min := parseBranchVersion(branch)
	if maj < 0 {
		return true
	}
	maxMaj, maxMin := parseBranchVersion(maxBranch)
	if maxMaj < 0 {
		return true
	}
	return maj < maxMaj || (maj == maxMaj && min <= maxMin)
}

func branchAtLeast(branch, minBranch string) bool {
	maj, min := parseBranchVersion(branch)
	if maj < 0 {
		return true // oadp-dev, main, etc. — always includes everything
	}
	minMaj, minMin := parseBranchVersion(minBranch)
	if minMaj < 0 {
		return true
	}
	return maj > minMaj || (maj == minMaj && min >= minMin)
}
