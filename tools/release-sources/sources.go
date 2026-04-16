package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// Source constants for display and config.
const (
	pyxisGitLab    = "gitlab.cee.redhat.com"
	pyxisProject   = "releng/pyxis-repo-configs"
	pyxisFilePath  = "products/oadp/oadp.yaml"
	pyxisBranch    = "main"
	obdOwner       = "openshift-eng"
	obdRepo        = "ocp-build-data"
	operatorOwner  = "openshift"
	operatorRepo   = "oadp-operator"
	imgRefsPath    = "bundle/image-references"
	konfluxProject = "releng/konflux-release-data"
	konfluxRPADir  = "config/kflux-ocp-p01.7ayg.p1/product/ReleasePlanAdmission/art-oadp"
)

// exceptions are repos skipped entirely from comparison (known special cases).
var exceptions = map[string]bool{
	"oadp/oadp-cli-rhel9":                                  true,
	"oadp/oadp-filebrowser-rhel9":                          true,
	"oadp/oadp-vmdp-rhel9":                                 true,
	"oadp/kubevirt-velero-plugin-rhel9":                     true,
	"oadp/oadp-operator-bundle":                             true,
	"oadp/oadp-kubevirt-velero-annotations-remover-rhel9":   true,
	"oadp/oadp-kubevirt-datamover-monitor-rhel9":            true,
}

// noPyxis are repos that are not expected to be in Pyxis (defined elsewhere).
var noPyxis = map[string]bool{
	"oadp/oadp-mustgather-rhel9":                       true,
	"oadp/oadp-non-admin-rhel9":                        true,
	"oadp/oadp-operator-bundle":                        true,
	"oadp/oadp-rhel9-operator":                         true,
	"oadp/oadp-velero-plugin-for-aws-rhel9":            true,
	"oadp/oadp-velero-plugin-for-gcp-rhel9":            true,
	"oadp/oadp-velero-plugin-for-legacy-aws-rhel9":     true,
	"oadp/oadp-velero-plugin-for-microsoft-azure-rhel9": true,
	"oadp/oadp-velero-plugin-rhel9":                    true,
	"oadp/oadp-velero-rhel9":                           true,
}

// branchSuffix converts "oadp-1.6" to "1-6" for Konflux advisory file names.
// Returns "" for non-versioned branches like "oadp-dev".
func branchSuffix(branch string) string {
	rest := strings.TrimPrefix(branch, "oadp-")
	if rest == branch {
		return ""
	}
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 {
		return ""
	}
	// Verify both parts are numeric
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return ""
			}
		}
	}
	return parts[0] + "-" + parts[1]
}

// FetchAll fetches all five sources and returns the combined result.
func FetchAll(gh *GitHubClient, gl *GitLabClient, branch string) *Sources {
	src := &Sources{
		OBDDelivery: make(map[string][]string),
		ImgRefFrom:  make(map[string]string),
	}

	suffix := branchSuffix(branch)

	// Fetch sequentially with progress output
	src.Pyxis = fetchPyxis(gl)
	src.OBD = fetchOBD(gh, branch, src.OBDDelivery)
	src.ImageRefs = fetchImageRefs(gh, branch, src.ImgRefFrom)
	if suffix != "" {
		src.Stage = fetchKonfluxAdvisory(gl, suffix, "stage")
		src.Prod = fetchKonfluxAdvisory(gl, suffix, "prod")
	} else {
		src.Stage = SourceData{Name: "Konflux stage", Detail: "(no versioned branch)"}
		src.Prod = SourceData{Name: "Konflux prod", Detail: "(no versioned branch)"}
	}

	return src
}

// fetchPyxis fetches the Pyxis repo config from GitLab.
func fetchPyxis(gl *GitLabClient) SourceData {
	sd := SourceData{
		Name:   "Pyxis",
		Repos:  make(map[string]bool),
		Detail: fmt.Sprintf("%s @ %s", pyxisProject, pyxisBranch),
	}

	progress("1/5", "Pyxis", sd.Detail)

	content, err := gl.FileContent(pyxisProject, pyxisFilePath, pyxisBranch)
	if err != nil {
		progressResult("(unavailable)")
		return sd
	}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "repository: oadp/") {
			repo := strings.TrimSpace(strings.SplitN(line, "repository: ", 2)[1])
			if repo != "" {
				sd.Repos[repo] = true
			}
		}
	}

	sd.Available = len(sd.Repos) > 0
	progressResult("")
	return sd
}

// fetchOBD fetches image names and delivery metadata from ocp-build-data in a single pass.
func fetchOBD(gh *GitHubClient, branch string, deliveryMap map[string][]string) SourceData {
	sd := SourceData{
		Name:   "ocp-build-data",
		Repos:  make(map[string]bool),
		Detail: fmt.Sprintf("%s/%s @ %s", obdOwner, obdRepo, branch),
	}

	progress("2/5", "ocp-build-data", sd.Detail)

	files, err := gh.DirListing(obdOwner, obdRepo, "images", branch)
	if err != nil || files == nil {
		progressResult("(unavailable)")
		return sd
	}

	type result struct {
		name     string
		delivery []string
	}
	ch := make(chan result, len(files))
	var wg sync.WaitGroup

	for _, f := range files {
		if f == "base-rhel9.yml" || (!strings.HasSuffix(f, ".yml") && !strings.HasSuffix(f, ".yaml")) {
			continue
		}
		wg.Add(1)
		go func(fname string) {
			defer wg.Done()
			content, err := gh.FileContent(obdOwner, obdRepo, "images/"+fname, branch)
			if err != nil || content == nil {
				return
			}
			s := string(content)
			name := parseOBDName(s)
			if name != "" {
				ch <- result{name: name, delivery: parseOBDDeliveryNames(s)}
			}
		}(f)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		sd.Repos[r.name] = true
		if len(r.delivery) > 0 {
			deliveryMap[r.name] = r.delivery
		}
	}

	sd.Available = len(sd.Repos) > 0
	progressResult("")
	return sd
}

// fetchImageRefs fetches image-references from oadp-operator on GitHub.
func fetchImageRefs(gh *GitHubClient, branch string, fromMap map[string]string) SourceData {
	sd := SourceData{
		Name:   "image-refs",
		Repos:  make(map[string]bool),
		Detail: fmt.Sprintf("%s/%s @ %s", operatorOwner, operatorRepo, branch),
	}

	progress("3/5", "image-refs", fmt.Sprintf("%s/%s @ %s  (%s)", operatorOwner, operatorRepo, branch, imgRefsPath))

	content, err := gh.FileContent(operatorOwner, operatorRepo, imgRefsPath, branch)
	if err != nil || content == nil {
		progressResult("(unavailable)")
		return sd
	}

	raw := string(content)

	// Parse "- name:" entries and pair with "name:" from-image URLs
	var currentName string
	lines := strings.Split(raw, "\n")
	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// Strip comment prefix
		line := trimmed
		if strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}

		if strings.HasPrefix(line, "- name:") {
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "- name:"))
			continue
		}

		if currentName == "" {
			continue
		}

		// Skip kind: and from: lines
		if strings.HasPrefix(line, "kind:") || strings.HasPrefix(line, "from:") {
			continue
		}

		// Inside from: block, capture "name:" (the image reference)
		if strings.HasPrefix(line, "name:") && strings.Contains(line, "quay") {
			fromImage := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			repoName := "oadp/" + currentName
			sd.Repos[repoName] = true
			if fromImage != "" {
				fromMap[repoName] = fromImage
			}
			currentName = ""
		}
	}

	sd.Available = len(sd.Repos) > 0
	progressResult("")
	return sd
}

// fetchKonfluxAdvisory fetches a Konflux ReleasePlanAdmission advisory from GitLab.
// kind is "stage" or "prod".
func fetchKonfluxAdvisory(gl *GitLabClient, suffix, kind string) SourceData {
	sd := SourceData{
		Name:   fmt.Sprintf("Konflux %s", kind),
		Repos:  make(map[string]bool),
		Detail: fmt.Sprintf("%s @ main  (oadp-advisory-%s-%s)", konfluxProject, kind, suffix),
	}

	step := "4/5"
	if kind == "prod" {
		step = "5/5"
	}
	progress(step, sd.Name, sd.Detail)

	filePath := fmt.Sprintf("%s/oadp-advisory-%s-%s.yaml", konfluxRPADir, kind, suffix)
	content, err := gl.FileContent(konfluxProject, filePath, "main")
	if err != nil {
		progressResult("(unavailable)")
		return sd
	}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "url: registry") {
			continue
		}
		// Extract repo path from "url: registry.redhat.io/oadp/oadp-velero-rhel9"
		idx := strings.Index(line, "url: registry")
		if idx < 0 {
			continue
		}
		urlPart := strings.TrimSpace(line[idx+len("url: "):])
		// Strip registry hostname: "registry.redhat.io/oadp/..." -> "oadp/..."
		// or "registry.stage.redhat.io/oadp/..." -> "oadp/..."
		if slashIdx := strings.Index(urlPart, "/"); slashIdx >= 0 {
			repo := urlPart[slashIdx+1:]
			if repo != "" {
				sd.Repos[repo] = true
			}
		}
	}

	sd.Available = len(sd.Repos) > 0
	progressResult("")
	return sd
}

// BuildUnion returns a sorted list of all repos across all sources,
// excluding repos in the exceptions list.
func BuildUnion(src *Sources) []string {
	all := make(map[string]bool)
	for _, sd := range []*SourceData{&src.Pyxis, &src.OBD, &src.ImageRefs, &src.Stage, &src.Prod} {
		for repo := range sd.Repos {
			all[repo] = true
		}
	}

	var repos []string
	for repo := range all {
		if !exceptions[repo] {
			repos = append(repos, repo)
		}
	}

	sort.Strings(repos)
	return repos
}

// sourceChecks defines the sources to check for each repo, in order.
func sourceChecks(src *Sources, repo string) []struct {
	sd    *SourceData
	label string
} {
	return []struct {
		sd    *SourceData
		label string
	}{
		{&src.Pyxis, "Pyxis"},
		{&src.OBD, "OBD"},
		{&src.ImageRefs, "ImgRef"},
		{&src.Stage, "Stage"},
		{&src.Prod, "Prod"},
	}
}

// FindIssues checks for cross-source gaps and metadata mismatches.
func FindIssues(src *Sources, repos []string) []Issue {
	var issues []Issue

	for _, repo := range repos {
		var missing, found []string
		for _, sc := range sourceChecks(src, repo) {
			if !sc.sd.Available || (sc.label == "Pyxis" && noPyxis[repo]) {
				continue
			}
			if sc.sd.Repos[repo] {
				found = append(found, sc.label)
			} else {
				missing = append(missing, sc.label)
			}
		}
		if len(missing) > 0 {
			issues = append(issues, Issue{
				Severity:  "error",
				Repo:      repo,
				Message:   fmt.Sprintf("missing from %s", strings.Join(missing, ", ")),
				InSources: strings.Join(found, ", "),
			})
		}
	}

	for repo, delivery := range src.OBDDelivery {
		if exceptions[repo] {
			continue
		}
		if !deliveryMatch(delivery, repo) {
			issues = append(issues, Issue{
				Severity: "warning",
				Repo:     repo,
				Message:  fmt.Sprintf("name/delivery_repo_names mismatch (delivery: %s)", strings.Join(delivery, ", ")),
			})
		}
	}

	return issues
}

// deliveryMatch returns true if repo appears in the delivery_repo_names list.
func deliveryMatch(delivery []string, repo string) bool {
	for _, d := range delivery {
		if d == repo {
			return true
		}
	}
	return false
}

// --- OBD YAML parsing (line-based, no external deps) ---

// parseOBDName extracts the top-level "name:" field from an ocp-build-data YAML.
func parseOBDName(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
	}
	return ""
}

// parseOBDDeliveryNames extracts the delivery_repo_names list from an ocp-build-data YAML.
func parseOBDDeliveryNames(content string) []string {
	var names []string
	inDelivery := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "delivery_repo_names:" {
			inDelivery = true
			continue
		}
		if inDelivery {
			if strings.HasPrefix(trimmed, "- ") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				names = append(names, name)
			} else if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				break
			}
		}
	}
	return names
}

// --- Helpers ---

func progress(step, name, detail string) {
	fmt.Fprintf(os.Stderr, "  %s[%s]%s %-16s %s%s%s",
		cDim, step, cReset, name, cDim, detail, cReset)
}

func progressResult(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", cYellow, msg, cReset)
	} else {
		fmt.Fprintln(os.Stderr)
	}
}

