package main

import (
	"strings"
	"testing"
)

func TestParseOwnersYAML_ValidFile(t *testing.T) {
	data := []byte(`approvers:
  - alice
  - bob
reviewers:
  - charlie
  - diana
`)
	info := parseOwnersYAML(data, "OWNERS")
	if !info.HasFile {
		t.Fatal("expected HasFile=true")
	}
	if info.FilePath != "OWNERS" {
		t.Errorf("FilePath = %q, want OWNERS", info.FilePath)
	}
	if len(info.Approvers) != 2 {
		t.Errorf("got %d approvers, want 2", len(info.Approvers))
	}
	if len(info.Reviewers) != 2 {
		t.Errorf("got %d reviewers, want 2", len(info.Reviewers))
	}
	if info.Approvers[0] != "alice" || info.Approvers[1] != "bob" {
		t.Errorf("approvers = %v, want [alice bob]", info.Approvers)
	}
}

func TestParseOwnersYAML_ApproversOnly(t *testing.T) {
	data := []byte(`approvers:
  - alice
`)
	info := parseOwnersYAML(data, "DOWNSTREAM_OWNERS")
	if !info.HasFile {
		t.Fatal("expected HasFile=true")
	}
	if info.FilePath != "DOWNSTREAM_OWNERS" {
		t.Errorf("FilePath = %q, want DOWNSTREAM_OWNERS", info.FilePath)
	}
	if len(info.Approvers) != 1 {
		t.Errorf("got %d approvers, want 1", len(info.Approvers))
	}
	if len(info.Reviewers) != 0 {
		t.Errorf("got %d reviewers, want 0", len(info.Reviewers))
	}
}

func TestParseOwnersYAML_EmptyFile(t *testing.T) {
	info := parseOwnersYAML([]byte(""), "OWNERS")
	if !info.HasFile {
		t.Fatal("expected HasFile=true for valid but empty YAML")
	}
	if len(info.Approvers) != 0 {
		t.Errorf("got %d approvers, want 0", len(info.Approvers))
	}
}

func TestParseOwnersYAML_InvalidYAML(t *testing.T) {
	info := parseOwnersYAML([]byte("{{invalid"), "OWNERS")
	if info.HasFile {
		t.Error("expected HasFile=false for invalid YAML")
	}
}

func TestAuditRepo_OwnersPresent(t *testing.T) {
	rc := &RepoConfig{
		Repo:      "openshift/velero",
		HasConfig: true,
		Owners: &OwnersInfo{
			HasFile:   true,
			FilePath:  "OWNERS",
			Approvers: []string{"alice", "bob"},
			Reviewers: []string{"charlie"},
		},
		PluginConfig: map[string]interface{}{
			"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
			"lgtm":    []interface{}{},
			"plugins": map[string]interface{}{
				"openshift/velero": []interface{}{"approve"},
			},
		},
		ProwConfig: map[string]interface{}{},
	}

	audit := AuditRepo(rc, "upstream-rebase")

	// Should have an OK finding for owners_approvers.
	found := false
	for _, f := range audit.Findings {
		if f.Field == "owners_approvers" && f.Severity == SeverityOK {
			found = true
			if !strings.Contains(f.Message, "2 approvers") {
				t.Errorf("expected '2 approvers' in message, got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected OK finding for owners_approvers")
	}

	// Owners info should be populated.
	if !audit.Owners.HasFile {
		t.Error("expected Owners.HasFile=true")
	}
	if len(audit.Owners.Approvers) != 2 {
		t.Errorf("expected 2 approvers, got %d", len(audit.Owners.Approvers))
	}
}

func TestAuditRepo_OwnersMissing(t *testing.T) {
	rc := &RepoConfig{
		Repo:      "openshift/velero",
		HasConfig: true,
		Owners: &OwnersInfo{
			HasFile: false,
		},
		PluginConfig: map[string]interface{}{
			"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
			"lgtm":    []interface{}{},
			"plugins": map[string]interface{}{
				"openshift/velero": []interface{}{"approve"},
			},
		},
		ProwConfig: map[string]interface{}{},
	}

	audit := AuditRepo(rc, "upstream-rebase")

	found := false
	for _, f := range audit.Findings {
		if f.Field == "owners" && f.Severity == SeverityWarning {
			found = true
			if !strings.Contains(f.Message, "No OWNERS") {
				t.Errorf("expected 'No OWNERS' in message, got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected warning finding for missing OWNERS")
	}
}

func TestAuditRepo_OwnersEmptyApprovers(t *testing.T) {
	rc := &RepoConfig{
		Repo:      "openshift/velero",
		HasConfig: true,
		Owners: &OwnersInfo{
			HasFile:   true,
			FilePath:  "OWNERS",
			Approvers: []string{},
			Reviewers: []string{"charlie"},
		},
		PluginConfig: map[string]interface{}{
			"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
			"lgtm":    []interface{}{},
			"plugins": map[string]interface{}{
				"openshift/velero": []interface{}{"approve"},
			},
		},
		ProwConfig: map[string]interface{}{},
	}

	audit := AuditRepo(rc, "upstream-rebase")

	found := false
	for _, f := range audit.Findings {
		if f.Field == "owners_approvers" && f.Severity == SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for empty approvers")
	}
}

func TestAuditRepo_OwnersNoReviewers(t *testing.T) {
	rc := &RepoConfig{
		Repo:      "openshift/velero",
		HasConfig: true,
		Owners: &OwnersInfo{
			HasFile:   true,
			FilePath:  "OWNERS",
			Approvers: []string{"alice"},
			Reviewers: []string{},
		},
		PluginConfig: map[string]interface{}{
			"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
			"lgtm":    []interface{}{},
			"plugins": map[string]interface{}{
				"openshift/velero": []interface{}{"approve"},
			},
		},
		ProwConfig: map[string]interface{}{},
	}

	audit := AuditRepo(rc, "upstream-rebase")

	found := false
	for _, f := range audit.Findings {
		if f.Field == "owners_reviewers" && f.Severity == SeverityInfo {
			found = true
		}
	}
	if !found {
		t.Error("expected info finding for empty reviewers")
	}
}

func TestAuditRepo_OwnersNilInConfig(t *testing.T) {
	// When Owners is nil in RepoConfig (e.g., from cache before feature existed).
	rc := &RepoConfig{
		Repo:      "openshift/velero",
		HasConfig: true,
		Owners:    nil,
		PluginConfig: map[string]interface{}{
			"approve": []interface{}{map[string]interface{}{"require_self_approval": false}},
			"lgtm":    []interface{}{},
			"plugins": map[string]interface{}{
				"openshift/velero": []interface{}{"approve"},
			},
		},
		ProwConfig: map[string]interface{}{},
	}

	audit := AuditRepo(rc, "upstream-rebase")

	// Should produce a warning about missing OWNERS since Owners is zero-value.
	found := false
	for _, f := range audit.Findings {
		if f.Field == "owners" && f.Severity == SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for nil Owners (treated as missing)")
	}
}

func TestRenderRepoText_IncludesOwners(t *testing.T) {
	report := &AuditReport{
		Groups: []RepoGroup{
			{
				Name: "Test",
				Type: "upstream-rebase",
				Repos: []RepoAudit{
					{
						Name:      "openshift/velero",
						HasConfig: true,
						Fields:    map[string]string{},
						Owners: OwnersInfo{
							HasFile:   true,
							FilePath:  "OWNERS",
							Approvers: []string{"alice", "bob"},
							Reviewers: []string{"charlie"},
						},
						Findings: []Finding{},
					},
				},
			},
		},
	}
	out := RenderRepoText(report, "openshift/velero")
	if !strings.Contains(out, "OWNERS: 2 approvers, 1 reviewers") {
		t.Errorf("expected OWNERS info in output, got:\n%s", out)
	}
}

func TestRenderRepoText_MissingOwners(t *testing.T) {
	report := &AuditReport{
		Groups: []RepoGroup{
			{
				Name: "Test",
				Type: "upstream-rebase",
				Repos: []RepoAudit{
					{
						Name:      "openshift/velero",
						HasConfig: true,
						Fields:    map[string]string{},
						Findings:  []Finding{},
					},
				},
			},
		},
	}
	out := RenderRepoText(report, "openshift/velero")
	if !strings.Contains(out, "OWNERS: missing") {
		t.Errorf("expected 'OWNERS: missing' in output, got:\n%s", out)
	}
}
