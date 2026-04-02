package main

import (
	"strings"
	"testing"
)

// helper to build a RepoConfig with common plugin/prow structures.
func makeRepoConfig(repo string, pluginConfig, prowConfig map[string]interface{}) *RepoConfig {
	rc := &RepoConfig{
		Repo:         repo,
		HasConfig:    true,
		PluginConfig: pluginConfig,
		ProwConfig:   prowConfig,
		Owners: &OwnersInfo{
			HasFile:   true,
			FilePath:  "OWNERS",
			Approvers: []string{"alice", "bob"},
			Reviewers: []string{"charlie"},
		},
	}
	return rc
}

// standard plugin config with approve+lgtm sections and approve plugin listed.
func standardPluginConfig(repo string) map[string]interface{} {
	return map[string]interface{}{
		"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
		"lgtm":    []interface{}{},
		"plugins": map[string]interface{}{
			repo: []interface{}{"approve"},
		},
	}
}

func findFinding(findings []Finding, field string) *Finding {
	for _, f := range findings {
		if f.Field == field {
			return &f
		}
	}
	return nil
}

func findFindings(findings []Finding, field string) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.Field == field {
			result = append(result, f)
		}
	}
	return result
}

func TestAuditRepo_NoConfig(t *testing.T) {
	rc := &RepoConfig{Repo: "openshift/velero", HasConfig: false}
	audit := AuditRepo(rc, "upstream-rebase")

	if audit.HasConfig {
		t.Error("expected HasConfig=false")
	}
	if len(audit.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(audit.Findings))
	}
	if audit.Findings[0].Severity != SeverityIssue {
		t.Errorf("expected SeverityIssue, got %v", audit.Findings[0].Severity)
	}
	if audit.Findings[0].Field != "config" {
		t.Errorf("expected field 'config', got %q", audit.Findings[0].Field)
	}
}

func TestAuditRepo_UpstreamRebase_ForcePushTrue(t *testing.T) {
	prowConfig := map[string]interface{}{
		"branch-protection": map[string]interface{}{
			"orgs": map[string]interface{}{
				"openshift": map[string]interface{}{
					"repos": map[string]interface{}{
						"velero": map[string]interface{}{
							"allow_force_pushes": true,
						},
					},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/velero", standardPluginConfig("openshift/velero"), prowConfig)
	audit := AuditRepo(rc, "upstream-rebase")

	f := findFinding(audit.Findings, "allow_force_pushes")
	if f == nil {
		t.Fatal("expected finding for allow_force_pushes")
	}
	if f.Severity != SeverityOK {
		t.Errorf("expected SeverityOK for force push=true on upstream-rebase, got %v", f.Severity)
	}
}

func TestAuditRepo_UpstreamRebase_ForcePushMissing(t *testing.T) {
	rc := makeRepoConfig("openshift/velero", standardPluginConfig("openshift/velero"), map[string]interface{}{})
	audit := AuditRepo(rc, "upstream-rebase")

	f := findFinding(audit.Findings, "allow_force_pushes")
	if f == nil {
		t.Fatal("expected finding for allow_force_pushes")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for missing force push on upstream-rebase, got %v", f.Severity)
	}
}

func TestAuditRepo_UpstreamRebase_EnforceAdminsTrue(t *testing.T) {
	prowConfig := map[string]interface{}{
		"branch-protection": map[string]interface{}{
			"orgs": map[string]interface{}{
				"openshift": map[string]interface{}{
					"repos": map[string]interface{}{
						"velero": map[string]interface{}{
							"enforce_admins":     true,
							"allow_force_pushes": true,
						},
					},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/velero", standardPluginConfig("openshift/velero"), prowConfig)
	audit := AuditRepo(rc, "upstream-rebase")

	f := findFinding(audit.Findings, "enforce_admins")
	if f == nil {
		t.Fatal("expected finding for enforce_admins on upstream-rebase")
	}
	if f.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for enforce_admins=true on upstream-rebase (unusual), got %v", f.Severity)
	}
}

func TestAuditRepo_OADPOwned_EnforceAdminsMissing(t *testing.T) {
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), map[string]interface{}{})
	audit := AuditRepo(rc, "oadp-owned-openshift")

	f := findFinding(audit.Findings, "enforce_admins")
	if f == nil {
		t.Fatal("expected finding for enforce_admins")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", f.Severity)
	}
	if !strings.Contains(f.Message, "tideErrLoopBlocker") {
		t.Errorf("expected message to mention tideErrLoopBlocker, got %q", f.Message)
	}
}

func TestAuditRepo_OADPOwned_ReviewCountMissing(t *testing.T) {
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), map[string]interface{}{})
	audit := AuditRepo(rc, "oadp-owned-openshift")

	f := findFinding(audit.Findings, "review_count")
	if f == nil {
		t.Fatal("expected finding for review_count")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", f.Severity)
	}
}

func TestAuditRepo_OADPOwned_ReviewCountNot2(t *testing.T) {
	prowConfig := map[string]interface{}{
		"branch-protection": map[string]interface{}{
			"orgs": map[string]interface{}{
				"openshift": map[string]interface{}{
					"repos": map[string]interface{}{
						"oadp-operator": map[string]interface{}{
							"enforce_admins": true,
							"required_pull_request_reviews": map[string]interface{}{
								"required_approving_review_count": 1,
								"dismiss_stale_reviews":           true,
							},
						},
					},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), prowConfig)
	audit := AuditRepo(rc, "oadp-owned-openshift")

	f := findFinding(audit.Findings, "review_count")
	if f == nil {
		t.Fatal("expected finding for review_count")
	}
	if f.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for review_count=1 (not 2), got %v", f.Severity)
	}
}

func TestAuditRepo_OADPOwned_FullyConfigured(t *testing.T) {
	prowConfig := map[string]interface{}{
		"branch-protection": map[string]interface{}{
			"orgs": map[string]interface{}{
				"openshift": map[string]interface{}{
					"repos": map[string]interface{}{
						"oadp-operator": map[string]interface{}{
							"enforce_admins": true,
							"required_pull_request_reviews": map[string]interface{}{
								"required_approving_review_count": 2,
								"dismiss_stale_reviews":           true,
							},
						},
					},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), prowConfig)
	audit := AuditRepo(rc, "oadp-owned-openshift")

	// Should NOT have warnings for enforce_admins, review_count, or dismiss_stale_reviews.
	for _, f := range audit.Findings {
		if f.Severity == SeverityWarning && (f.Field == "enforce_admins" || f.Field == "review_count" || f.Field == "dismiss_stale_reviews") {
			t.Errorf("unexpected warning for %s: %s", f.Field, f.Message)
		}
	}
}

func TestAuditRepo_OADPOwned_ForcePushTrue(t *testing.T) {
	prowConfig := map[string]interface{}{
		"branch-protection": map[string]interface{}{
			"orgs": map[string]interface{}{
				"openshift": map[string]interface{}{
					"repos": map[string]interface{}{
						"oadp-operator": map[string]interface{}{
							"allow_force_pushes": true,
						},
					},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), prowConfig)
	audit := AuditRepo(rc, "oadp-owned-openshift")

	f := findFinding(audit.Findings, "allow_force_pushes")
	if f == nil {
		t.Fatal("expected finding for allow_force_pushes on oadp-owned")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for force push=true on oadp-owned (unexpected), got %v", f.Severity)
	}
}

func TestAuditRepo_OADPOwned_DismissStaleReviewsMissing(t *testing.T) {
	rc := makeRepoConfig("openshift/oadp-operator", standardPluginConfig("openshift/oadp-operator"), map[string]interface{}{})
	audit := AuditRepo(rc, "oadp-owned-openshift")

	f := findFinding(audit.Findings, "dismiss_stale_reviews")
	if f == nil {
		t.Fatal("expected finding for dismiss_stale_reviews")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", f.Severity)
	}
}

func TestAuditRepo_MissingPlugins(t *testing.T) {
	// No approve section, no lgtm section, no approve plugin.
	pluginConfig := map[string]interface{}{
		"plugins": map[string]interface{}{
			"openshift/velero": []interface{}{"trigger"},
		},
	}
	rc := makeRepoConfig("openshift/velero", pluginConfig, map[string]interface{}{})
	audit := AuditRepo(rc, "upstream-rebase")

	approveSection := findFinding(audit.Findings, "approve_section")
	if approveSection == nil || approveSection.Severity != SeverityIssue {
		t.Error("expected SeverityIssue for missing approve section")
	}

	lgtmSection := findFinding(audit.Findings, "lgtm_section")
	if lgtmSection == nil || lgtmSection.Severity != SeverityIssue {
		t.Error("expected SeverityIssue for missing lgtm section")
	}

	approvePlugin := findFinding(audit.Findings, "approve_plugin")
	if approvePlugin == nil || approvePlugin.Severity != SeverityIssue {
		t.Error("expected SeverityIssue for missing approve plugin")
	}
}

func TestAuditRepo_RequireSelfApproval_NotSet(t *testing.T) {
	pluginConfig := map[string]interface{}{
		"approve": []interface{}{map[string]interface{}{}}, // no require_self_approval
		"lgtm":    []interface{}{},
		"plugins": map[string]interface{}{
			"openshift/velero": []interface{}{"approve"},
		},
	}
	rc := makeRepoConfig("openshift/velero", pluginConfig, map[string]interface{}{})
	audit := AuditRepo(rc, "upstream-rebase")

	f := findFinding(audit.Findings, "require_self_approval")
	if f == nil {
		t.Fatal("expected finding for require_self_approval")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for NOT_SET, got %v", f.Severity)
	}
}

func TestAuditRepo_RequireSelfApproval_True(t *testing.T) {
	pluginConfig := map[string]interface{}{
		"approve": []interface{}{map[string]interface{}{"require_self_approval": true}},
		"lgtm":    []interface{}{},
		"plugins": map[string]interface{}{
			"openshift/velero": []interface{}{"approve"},
		},
	}
	rc := makeRepoConfig("openshift/velero", pluginConfig, map[string]interface{}{})
	audit := AuditRepo(rc, "upstream-rebase")

	f := findFinding(audit.Findings, "require_self_approval")
	if f == nil {
		t.Fatal("expected finding for require_self_approval")
	}
	if f.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for require_self_approval=true, got %v", f.Severity)
	}
}

func TestAuditRepo_MigtoolsRepoType(t *testing.T) {
	// oadp-owned-migtools should follow same rules as oadp-owned-openshift.
	rc := makeRepoConfig("migtools/oadp-non-admin", standardPluginConfig("migtools/oadp-non-admin"), map[string]interface{}{})
	audit := AuditRepo(rc, "oadp-owned-migtools")

	f := findFinding(audit.Findings, "enforce_admins")
	if f == nil {
		t.Fatal("expected finding for enforce_admins")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for missing enforce_admins on migtools, got %v", f.Severity)
	}
}

func TestAuditRepo_TideLabelsCheck(t *testing.T) {
	prowConfig := map[string]interface{}{
		"tide": map[string]interface{}{
			"queries": []interface{}{
				map[string]interface{}{
					"repos":           []interface{}{"openshift/velero"},
					"includedBranches": []interface{}{"main"},
					"labels":          []interface{}{"approved"}, // missing "lgtm"
					"missingLabels":   []interface{}{"do-not-merge/hold"},
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/velero", standardPluginConfig("openshift/velero"), prowConfig)
	audit := AuditRepo(rc, "upstream-rebase")

	// Should warn about missing "lgtm" label in tide query.
	found := false
	for _, f := range audit.Findings {
		if f.Field == "tide_labels" && strings.Contains(f.Message, "lgtm") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about missing 'lgtm' in Tide required labels")
	}
}

func TestAuditRepo_TideMissingLabelsCheck(t *testing.T) {
	prowConfig := map[string]interface{}{
		"tide": map[string]interface{}{
			"queries": []interface{}{
				map[string]interface{}{
					"repos":           []interface{}{"openshift/velero"},
					"includedBranches": []interface{}{"main"},
					"labels":          []interface{}{"approved", "lgtm"},
					"missingLabels":   []interface{}{}, // missing blocker labels
				},
			},
		},
	}
	rc := makeRepoConfig("openshift/velero", standardPluginConfig("openshift/velero"), prowConfig)
	audit := AuditRepo(rc, "upstream-rebase")

	missingLabelFindings := findFindings(audit.Findings, "tide_missing_labels")
	if len(missingLabelFindings) == 0 {
		t.Error("expected warnings about missing blocker labels in Tide query")
	}
	// Should warn about do-not-merge/hold at minimum.
	found := false
	for _, f := range missingLabelFindings {
		if strings.Contains(f.Message, "do-not-merge/hold") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about missing 'do-not-merge/hold' in Tide missing labels")
	}
}
