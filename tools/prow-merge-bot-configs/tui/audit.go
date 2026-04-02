package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// AuditRepo audits a single repository's prow configuration.
// repoType is one of: upstream-rebase, oadp-owned-openshift, oadp-owned-migtools.
func AuditRepo(rc *RepoConfig, repoType string) *RepoAudit {
	audit := &RepoAudit{
		Name:      rc.Repo,
		HasConfig: rc.HasConfig,
	}

	if !rc.HasConfig {
		audit.Findings = []Finding{
			{Severity: SeverityIssue, Field: "config", Message: "No prow config found"},
		}
		return audit
	}

	fields, plugins, tide := ExtractFields(rc)
	audit.Fields = fields
	audit.Plugins = plugins
	audit.Tide = tide

	// Populate OWNERS info from config.
	if rc.Owners != nil {
		audit.Owners = *rc.Owners
	}

	// Split repo into org/repoName for YAML snippet suggestions.
	parts := strings.SplitN(rc.Repo, "/", 2)
	org := parts[0]
	repoName := rc.Repo
	if len(parts) == 2 {
		repoName = parts[1]
	}

	// --- Plugin config checks ---

	if !plugins.HasApproveSection {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityIssue,
			Field:    "approve_section",
			Message:  "Missing top-level 'approve:' config section in _pluginconfig.yaml",
			SuggestedFix: fmt.Sprintf("# Add to _pluginconfig.yaml\napprove:\n- repos:\n  - %s/%s\n  require_self_approval: false", org, repoName),
		})
	}

	if !plugins.HasLgtmSection {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityIssue,
			Field:    "lgtm_section",
			Message:  "Missing top-level 'lgtm:' config section in _pluginconfig.yaml",
			SuggestedFix: fmt.Sprintf("# Add to _pluginconfig.yaml\nlgtm:\n- repos:\n  - %s/%s\n  review_acts_as_lgtm: false", org, repoName),
		})
	}

	if !plugins.HasApprovePlugin {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityIssue,
			Field:    "approve_plugin",
			Message:  "'approve' not listed in plugins",
			SuggestedFix: fmt.Sprintf("# Add to _pluginconfig.yaml under plugins:\nplugins:\n  %s/%s:\n    plugins:\n    - approve\n    - lgtm", org, repoName),
		})
	}

	// require_self_approval
	selfApproval := fields["require_self_approval"]
	selfApprovalFix := fmt.Sprintf("# In _pluginconfig.yaml approve section:\napprove:\n- repos:\n  - %s/%s\n  require_self_approval: false", org, repoName)
	if selfApproval == "MISSING_FILE" {
		audit.Findings = append(audit.Findings, Finding{
			Severity:     SeverityIssue,
			Field:        "require_self_approval",
			Message:      "_pluginconfig.yaml not found",
			SuggestedFix: selfApprovalFix,
		})
	} else if selfApproval == "NOT_SET" {
		audit.Findings = append(audit.Findings, Finding{
			Severity:     SeverityWarning,
			Field:        "require_self_approval",
			Message:      "require_self_approval not explicitly set (defaults vary)",
			SuggestedFix: selfApprovalFix,
		})
	} else if selfApproval != "false" {
		audit.Findings = append(audit.Findings, Finding{
			Severity:     SeverityInfo,
			Field:        "require_self_approval",
			Message:      fmt.Sprintf("require_self_approval=%s (most OADP repos use false)", selfApproval),
			SuggestedFix: selfApprovalFix,
		})
	}

	// review_acts_as_lgtm
	if fields["review_acts_as_lgtm"] == "true" {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityInfo,
			Field:    "review_acts_as_lgtm",
			Message:  "review_acts_as_lgtm=true (GitHub Approve review counts as /lgtm)",
		})
	}

	// --- Branch protection checks ---

	forcePush := fields["allow_force_pushes"]
	enforce := fields["enforce_admins"]
	reviewCount := fields["required_approving_review_count"]
	dismissStale := fields["dismiss_stale_reviews"]

	switch repoType {
	case "upstream-rebase":
		if forcePush != "true" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "allow_force_pushes",
				Message:  "missing allow_force_pushes=true (needed for rebasebot)",
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml branch-protection:\nbranch-protection:\n  orgs:\n    %s:\n      repos:\n        %s:\n          branches:\n            main:\n              protect: true\n              allow-force-pushes: true", org, repoName),
			})
		} else {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityOK,
				Field:    "allow_force_pushes",
				Message:  "allow_force_pushes=true (expected for upstream rebase repo)",
			})
		}
		if enforce == "true" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityInfo,
				Field:    "enforce_admins",
				Message:  "enforce_admins=true on upstream rebase repo (unusual)",
			})
		}
		if reviewCount != "NOT_SET" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityInfo,
				Field:    "review_count",
				Message:  fmt.Sprintf("required_approving_review_count=%s on upstream rebase repo", reviewCount),
			})
		}

	case "oadp-owned-openshift", "oadp-owned-migtools":
		if enforce != "true" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "enforce_admins",
				Message:  "Missing enforce_admins=true — Tide bypasses review count (tideErrLoopBlocker, prow#134)",
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml branch-protection:\nbranch-protection:\n  orgs:\n    %s:\n      repos:\n        %s:\n          enforce_admins: true", org, repoName),
			})
		}
		if reviewCount == "NOT_SET" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "review_count",
				Message:  "Missing required_approving_review_count",
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml branch-protection:\nbranch-protection:\n  orgs:\n    %s:\n      repos:\n        %s:\n          required_pull_request_reviews:\n            required_approving_review_count: 2", org, repoName),
			})
		} else if reviewCount != "2" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityInfo,
				Field:    "review_count",
				Message:  fmt.Sprintf("required_approving_review_count=%s (most use 2)", reviewCount),
			})
		}
		if dismissStale != "true" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "dismiss_stale_reviews",
				Message:  "Missing dismiss_stale_reviews=true",
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml branch-protection:\nbranch-protection:\n  orgs:\n    %s:\n      repos:\n        %s:\n          required_pull_request_reviews:\n            dismiss_stale_reviews: true", org, repoName),
			})
		}
		if forcePush == "true" {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "allow_force_pushes",
				Message:  "allow_force_pushes=true on OADP-owned repo (unexpected)",
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml, remove or set:\nbranch-protection:\n  orgs:\n    %s:\n      repos:\n        %s:\n          branches:\n            main:\n              allow-force-pushes: false", org, repoName),
			})
		}
	}

	// --- Merge method ---
	mergeMethod := fields["merge_method"]
	if mergeMethod != "NOT_SET" && mergeMethod != "" {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityInfo,
			Field:    "merge_method",
			Message:  fmt.Sprintf("merge_method=%s", mergeMethod),
		})
	}

	// --- keep-main-query-separate ---
	if tide.HasKeepMainQuerySeparate {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityInfo,
			Field:    "keep_main_query_separate",
			Message:  "Uses keep-main-query-separate label in Tide missingLabels",
		})
	}

	// --- Tide required labels ---
	expectedRequired := []string{"approved", "lgtm"}
	for _, label := range expectedRequired {
		found := false
		for _, l := range tide.RequiredLabels {
			if l == label {
				found = true
				break
			}
		}
		if !found && len(tide.IncludedBranches) > 0 {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "tide_labels",
				Message:  fmt.Sprintf("Tide query missing required label: %s", label),
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml tide queries:\ntide:\n  queries:\n  - labels:\n    - lgtm\n    - approved\n    repos:\n    - %s/%s", org, repoName),
			})
		}
	}

	// --- Tide missing/blocker labels ---
	expectedMissing := []string{
		"do-not-merge/hold",
		"do-not-merge/work-in-progress",
		"do-not-merge/invalid-owners-file",
		"needs-rebase",
		"jira/invalid-bug",
		"backports/unvalidated-commits",
	}
	for _, label := range expectedMissing {
		found := false
		for _, l := range tide.MissingLabels {
			if l == label {
				found = true
				break
			}
		}
		if !found && len(tide.IncludedBranches) > 0 {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "tide_missing_labels",
				Message:  fmt.Sprintf("Tide query missing blocker label: %s", label),
				SuggestedFix: fmt.Sprintf("# In _prowconfig.yaml tide queries:\ntide:\n  queries:\n  - missingLabels:\n    - do-not-merge/hold\n    - do-not-merge/work-in-progress\n    - do-not-merge/invalid-owners-file\n    - needs-rebase\n    - jira/invalid-bug\n    - backports/unvalidated-commits\n    repos:\n    - %s/%s", org, repoName),
			})
		}
	}

	// --- OWNERS file checks ---
	if !audit.Owners.HasFile {
		audit.Findings = append(audit.Findings, Finding{
			Severity: SeverityWarning,
			Field:    "owners",
			Message:  "No OWNERS or DOWNSTREAM_OWNERS file found in repo root",
		})
	} else {
		if len(audit.Owners.Approvers) == 0 {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityWarning,
				Field:    "owners_approvers",
				Message:  fmt.Sprintf("%s has no approvers listed", audit.Owners.FilePath),
			})
		} else {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityOK,
				Field:    "owners_approvers",
				Message:  fmt.Sprintf("%s: %d approvers", audit.Owners.FilePath, len(audit.Owners.Approvers)),
			})
		}
		if len(audit.Owners.Reviewers) == 0 {
			audit.Findings = append(audit.Findings, Finding{
				Severity: SeverityInfo,
				Field:    "owners_reviewers",
				Message:  fmt.Sprintf("%s has no reviewers listed (approvers used as fallback)", audit.Owners.FilePath),
			})
		}
	}

	return audit
}

// CompareField compares a single field across repos and returns the comparison
// result. Returns nil if all repos are consistent (no outliers).
func CompareField(fieldName string, configs map[string]*RepoConfig, repos []string) *FieldComparison {
	type repoValue struct {
		repo  string
		value string
	}

	var pairs []repoValue
	for _, repo := range repos {
		rc, ok := configs[repo]
		if !ok || !rc.HasConfig {
			continue
		}
		fields, _, _ := ExtractFields(rc)
		val, exists := fields[fieldName]
		if !exists {
			val = "NOT_SET"
		}
		pairs = append(pairs, repoValue{repo: repo, value: val})
	}

	if len(pairs) == 0 {
		return nil
	}

	// Count occurrences of each value.
	counts := make(map[string]int)
	for _, p := range pairs {
		counts[p.value]++
	}

	// Find the majority value (highest count).
	var majority string
	maxCount := 0
	for val, count := range counts {
		if count > maxCount {
			maxCount = count
			majority = val
		}
	}

	// Collect outliers.
	var outliers []Outlier
	for _, p := range pairs {
		if p.value != majority {
			outliers = append(outliers, Outlier{Repo: p.repo, Value: p.value})
		}
	}

	if len(outliers) == 0 {
		return nil
	}

	return &FieldComparison{
		Majority: majority,
		Outliers: outliers,
	}
}

// fieldsForGroupType returns the fields to compare for a given group type.
func fieldsForGroupType(groupType string) []string {
	switch groupType {
	case "upstream-rebase":
		return []string{
			"require_self_approval",
			"review_acts_as_lgtm",
			"allow_force_pushes",
			"merge_method",
		}
	case "oadp-owned-openshift", "oadp-owned-migtools":
		return []string{
			"require_self_approval",
			"review_acts_as_lgtm",
			"enforce_admins",
			"required_approving_review_count",
			"dismiss_stale_reviews",
			"allow_force_pushes",
			"merge_method",
		}
	default:
		return nil
	}
}

// RunAudit runs the full audit across all repo groups and returns a report.
func RunAudit(branch, localPath, token string) (*AuditReport, map[string]*RepoConfig, error) {
	// Determine source label.
	var source string
	if localPath != "" {
		source = localPath + " (local checkout)"
	} else {
		source = fmt.Sprintf("github.com/openshift/release @ %s", branch)
	}

	groups := DefaultGroups()

	// Collect all repos from all groups.
	var allRepos []string
	for _, g := range groups {
		allRepos = append(allRepos, g.Repos...)
	}

	// Fetch all configs concurrently.
	configs := FetchAllConfigs(allRepos, branch, localPath, token)

	// Audit each group.
	var reportGroups []RepoGroup
	for _, g := range groups {
		rg := RepoGroup{
			Name:        g.Name,
			Type:        g.Type,
			Description: g.Description,
		}
		for _, repo := range g.Repos {
			rc, ok := configs[repo]
			if !ok {
				rc = &RepoConfig{Repo: repo, HasConfig: false}
			}
			ra := AuditRepo(rc, g.Type)
			rg.Repos = append(rg.Repos, *ra)
		}
		reportGroups = append(reportGroups, rg)
	}

	// Run cross-repo comparison for each group.
	comparison := make(map[string]map[string]FieldComparison)
	for _, g := range groups {
		compareFields := fieldsForGroupType(g.Type)
		if len(compareFields) == 0 {
			continue
		}
		groupComparisons := make(map[string]FieldComparison)
		for _, fieldName := range compareFields {
			fc := CompareField(fieldName, configs, g.Repos)
			if fc != nil {
				groupComparisons[fieldName] = *fc
			}
		}
		if len(groupComparisons) > 0 {
			comparison[g.Type] = groupComparisons
		}
	}

	// Compute summary.
	var summary Summary
	for _, rg := range reportGroups {
		for _, ra := range rg.Repos {
			issues, warnings, infos := ra.CountBySeverity()
			summary.Issues += issues
			summary.Warnings += warnings
			summary.Info += infos
		}
	}

	// Fetch rate limit after all API calls are done so it reflects current usage.
	rateLimit := fetchRateLimitFn()

	return &AuditReport{
		Source:     source,
		Timestamp:  time.Now().UTC(),
		RateLimit:  rateLimit,
		Groups:     reportGroups,
		Comparison: comparison,
		Summary:    summary,
	}, configs, nil
}

// fetchRateLimitFn is the function used to fetch rate limit info.
// It can be replaced in tests to inject mock responses.
var fetchRateLimitFn = fetchRateLimit

// fetchRateLimit calls `gh api rate_limit` and parses the response.
func fetchRateLimit() *RateLimit {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	out, err := exec.Command("gh", "api", "rate_limit").Output()
	if err != nil {
		return nil
	}
	return parseRateLimitJSON(out)
}

// parseRateLimitJSON parses the JSON response from GitHub's rate_limit API.
func parseRateLimitJSON(data []byte) *RateLimit {
	var resp struct {
		Resources struct {
			Core struct {
				Remaining int   `json:"remaining"`
				Limit     int   `json:"limit"`
				Reset     int64 `json:"reset"`
			} `json:"core"`
			GraphQL struct {
				Remaining int   `json:"remaining"`
				Limit     int   `json:"limit"`
				Reset     int64 `json:"reset"`
			} `json:"graphql"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	return &RateLimit{
		Remaining:        resp.Resources.Core.Remaining,
		Limit:            resp.Resources.Core.Limit,
		ResetAt:          time.Unix(resp.Resources.Core.Reset, 0).UTC(),
		GraphQLRemaining: resp.Resources.GraphQL.Remaining,
		GraphQLLimit:     resp.Resources.GraphQL.Limit,
		GraphQLResetAt:   time.Unix(resp.Resources.GraphQL.Reset, 0).UTC(),
	}
}
