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

// Wave metadata — update this when adding new waves.
var wavesMeta = []WaveInfo{
	{1, "Independent Dependencies"},
	{2, "Velero Integration"},
	{3, "Plugins and Operator"},
	{4, "Downstream Controllers"},
	{5, "Final Dependents"},
}

// RepoDef is the canonical definition of a repo in the OADP ecosystem.
// This is the source of truth — config files enrich it, not define it.
type RepoDef struct {
	Org      string
	Repo     string
	Wave     int
	MainOnly bool   // only tracks main, never branched (e.g. udistribution)
	NoRebase bool   // not managed by rebasebot; track deps/build only (e.g. hypershift-oadp-plugin)
	Branch   string // override branch (default: use CLI branch, or "main" for MainOnly/NoRebase)
	MinBranch string // earliest release branch this repo appears in (e.g. "oadp-1.6"); empty = all branches
}

// allRepos is the complete catalog of OADP repositories.
// Add new repos here when onboarding them.
var allRepos = []RepoDef{
	// Wave 1 — base dependencies
	{Org: "migtools", Repo: "kopia", Wave: 1},
	{Org: "openshift", Repo: "restic", Wave: 1},
	{Org: "migtools", Repo: "filebrowser", Wave: 1, MinBranch: "oadp-1.6"},
	{Org: "migtools", Repo: "udistribution", Wave: 1, MainOnly: true},
	{Org: "migtools", Repo: "kubevirt-velero-plugin", Wave: 3},
	{Org: "migtools", Repo: "oadp-vmdp", Wave: 1, MinBranch: "oadp-1.6"},

	// Wave 2 — velero
	{Org: "openshift", Repo: "velero", Wave: 2},

	// Wave 3 — plugins + operator
	{Org: "openshift", Repo: "hypershift-oadp-plugin", Wave: 3, NoRebase: true, Branch: "main"},
	{Org: "openshift", Repo: "velero-plugin-for-csi", Wave: 3, MainOnly: true},
	{Org: "openshift", Repo: "oadp-operator", Wave: 3},
	{Org: "openshift", Repo: "velero-plugin-for-aws", Wave: 3},
	{Org: "openshift", Repo: "velero-plugin-for-legacy-aws", Wave: 3},
	{Org: "openshift", Repo: "velero-plugin-for-microsoft-azure", Wave: 3},
	{Org: "openshift", Repo: "velero-plugin-for-gcp", Wave: 3},

	// Wave 4 — downstream controllers
	{Org: "migtools", Repo: "oadp-non-admin", Wave: 4},
	{Org: "openshift", Repo: "openshift-velero-plugin", Wave: 4},
	{Org: "migtools", Repo: "kubevirt-datamover-controller", Wave: 4, MinBranch: "oadp-1.6"},
	{Org: "migtools", Repo: "oadp-vm-file-restore", Wave: 4, MinBranch: "oadp-1.6"},

	// Wave 5 — final dependents
	{Org: "openshift", Repo: "oadp-must-gather", Wave: 5},
	{Org: "migtools", Repo: "oadp-cli", Wave: 5, MinBranch: "oadp-1.6"},
	{Org: "migtools", Repo: "kubevirt-datamover-plugin", Wave: 5, MinBranch: "oadp-1.6"},
}

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
	{ID: "go_version", Header: "Go", CheckIDs: []string{"go_version"}},
	{ID: "ci_config", Header: "CI", CheckIDs: []string{"ci_config"}},
	{ID: "dep_sync", Header: "Deps", CheckIDs: []string{"dep_sync"}},
	{ID: "quay", Header: "Quay", CheckIDs: []string{"upstream_image", "productized"}},
	{ID: "konflux", Header: "Konflux", CheckIDs: []string{"konflux", "art_config"}},
}

// QuayImage describes a container image on Quay.io.
type QuayImage struct {
	Namespace string // e.g. "konveyor"
	Repo      string // e.g. "velero"
	Name      string // display name, e.g. "velero"
}

// repoImages maps "org/repo" to the list of Quay images that repo produces.
// Source of truth: oadp-operator CSV relatedImages + Makefile targets.
var repoImages = map[string][]QuayImage{
	"openshift/velero":                             {{Namespace: "konveyor", Repo: "velero", Name: "velero"}},
	"openshift/oadp-operator":                      {{Namespace: "konveyor", Repo: "oadp-operator", Name: "oadp-operator"}},
	"openshift/openshift-velero-plugin":            {{Namespace: "konveyor", Repo: "openshift-velero-plugin", Name: "openshift-velero-plugin"}},
	"openshift/velero-plugin-for-aws":              {{Namespace: "konveyor", Repo: "velero-plugin-for-aws", Name: "velero-plugin-for-aws"}},
	"openshift/velero-plugin-for-legacy-aws":       {{Namespace: "konveyor", Repo: "velero-plugin-for-legacy-aws", Name: "velero-plugin-for-legacy-aws"}},
	"openshift/velero-plugin-for-microsoft-azure":  {{Namespace: "konveyor", Repo: "velero-plugin-for-microsoft-azure", Name: "velero-plugin-for-microsoft-azure"}},
	"openshift/velero-plugin-for-gcp":              {{Namespace: "konveyor", Repo: "velero-plugin-for-gcp", Name: "velero-plugin-for-gcp"}},
	"migtools/kubevirt-velero-plugin":              {{Namespace: "konveyor", Repo: "kubevirt-velero-plugin", Name: "kubevirt-velero-plugin"}},
	"openshift/oadp-must-gather":                   {{Namespace: "konveyor", Repo: "oadp-must-gather", Name: "oadp-must-gather"}},
	"migtools/oadp-non-admin":                      {{Namespace: "konveyor", Repo: "oadp-non-admin", Name: "oadp-non-admin"}},
	"migtools/oadp-cli":                            {{Namespace: "konveyor", Repo: "oadp-cli-binaries", Name: "oadp-cli-binaries"}},
	"migtools/kubevirt-datamover-controller":       {{Namespace: "konveyor", Repo: "kubevirt-datamover-controller", Name: "kubevirt-datamover-controller"}},
	"migtools/kubevirt-datamover-plugin":           {{Namespace: "konveyor", Repo: "kubevirt-datamover-plugin", Name: "kubevirt-datamover-plugin"}},
	"migtools/oadp-vmdp":                          {{Namespace: "konveyor", Repo: "oadp-vmdp-binaries", Name: "oadp-vmdp-binaries"}},
	// filebrowser repo builds the vmfr-access-filebrowser sidecar container
	"migtools/filebrowser":                         {{Namespace: "konveyor", Repo: "oadp-vmfr-access-filebrowser", Name: "oadp-vmfr-access-filebrowser"}},
	// oadp-vm-file-restore produces 3 images from one repo
	"migtools/oadp-vm-file-restore": {
		{Namespace: "konveyor", Repo: "oadp-vm-file-restore", Name: "oadp-vm-file-restore"},
		{Namespace: "konveyor", Repo: "oadp-vmfr-access", Name: "oadp-vmfr-access"},
		{Namespace: "konveyor", Repo: "oadp-vmfr-access-sshd", Name: "oadp-vmfr-access-sshd"},
	},
}

// filenameToRepo maps config filename prefixes to org/repo.
var filenameToRepo = map[string]struct{ org, repo string }{
	"openshift_velero":                             {"openshift", "velero"},
	"openshift_restic":                             {"openshift", "restic"},
	"openshift_oadp-operator":                      {"openshift", "oadp-operator"},
	"openshift_oadp_must_gather":                   {"openshift", "oadp-must-gather"},
	"openshift_openshift_velero_plugin":            {"openshift", "openshift-velero-plugin"},
	"openshift_velero_plugin_for_aws":              {"openshift", "velero-plugin-for-aws"},
	"openshift_velero_plugin_for_gcp":              {"openshift", "velero-plugin-for-gcp"},
	"openshift_velero_plugin_for_csi":              {"openshift", "velero-plugin-for-csi"},
	"openshift_velero_plugin_for_legacy_aws":       {"openshift", "velero-plugin-for-legacy-aws"},
	"openshift_velero_plugin_for_microsoft_azure":  {"openshift", "velero-plugin-for-microsoft-azure"},
	"migtools_kopia":                               {"migtools", "kopia"},
	"migtools_filebrowser":                         {"migtools", "filebrowser"},
	"migtools_udistribution":                       {"migtools", "udistribution"},
	"migtools_kubevirt_velero_plugin":              {"migtools", "kubevirt-velero-plugin"},
	"migtools_oadp_non_admin":                      {"migtools", "oadp-non-admin"},
	"migtools_oadp_cli":                            {"migtools", "oadp-cli"},
	"migtools_kubevirt_datamover_controller":       {"migtools", "kubevirt-datamover-controller"},
	"migtools_kubevirt_datamover_plugin":           {"migtools", "kubevirt-datamover-plugin"},
	"migtools_oadp_vm_file_restore":               {"migtools", "oadp-vm-file-restore"},
	"migtools_oadp_vmdp":                          {"migtools", "oadp-vmdp"},
}

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
		// Skip main-only repos when checking a release branch
		if def.MainOnly && branch != "oadp-dev" {
			continue
		}

		// Skip repos that weren't introduced until a later release
		if def.MinBranch != "" && !branchAtLeast(branch, def.MinBranch) {
			continue
		}

		key := def.Org + "/" + def.Repo
		spec := RepoSpec{
			Org:    def.Org,
			Repo:   def.Repo,
			Branch: branch,
			Wave:   def.Wave,
		}

		// For main-only repos, the actual branch is "main"
		if def.MainOnly {
			spec.Branch = "main"
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

// expandVars resolves $VAR and ${VAR} references within values.
func expandVars(vars map[string]string) {
	varRefRe := regexp.MustCompile(`\$\{?([A-Z_][A-Z_0-9]*)\}?`)

	for pass := 0; pass < 3; pass++ {
		changed := false
		for k, v := range vars {
			expanded := varRefRe.ReplaceAllStringFunc(v, func(match string) string {
				name := varRefRe.FindStringSubmatch(match)[1]
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

// branchAtLeast returns true if branch >= minBranch in version ordering.
// Non-release branches (oadp-dev, main) always return true (they track tip).
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
