package main

import (
	"fmt"
	"time"
)

// Repo groups — matches audit.sh lines 49-83.
var (
	UpstreamRebaseRepos = []string{
		"openshift/velero",
		"openshift/velero-plugin-for-aws",
		"openshift/velero-plugin-for-legacy-aws",
		"openshift/velero-plugin-for-gcp",
		"openshift/velero-plugin-for-microsoft-azure",
		"openshift/velero-plugin-for-csi",
		"openshift/restic",
	}

	OADPOwnedOpenshiftRepos = []string{
		"openshift/oadp-operator",
		"openshift/oadp-must-gather",
		"openshift/openshift-velero-plugin",
		"openshift/hypershift-oadp-plugin",
	}

	OADPOwnedMigtoolsRepos = []string{
		"migtools/filebrowser",
		"migtools/kubevirt-datamover-controller",
		"migtools/kubevirt-datamover-plugin",
		"migtools/kubevirt-velero-plugin",
		"migtools/oadp-cli",
		"migtools/oadp-non-admin",
		"migtools/oadp-vm-file-restore",
		"migtools/udistribution",
		"migtools/velero-plugin-for-vsm",
		"migtools/volume-snapshot-mover",
		"migtools/oadp-vmdp",
		"migtools/kopia",
	}
)

// RepoGroupDef defines a group of repos and its audit expectations.
type RepoGroupDef struct {
	Name        string
	Type        string // upstream-rebase, oadp-owned-openshift, oadp-owned-migtools
	Description string
	Repos       []string
}

// DefaultGroups returns the standard group definitions.
func DefaultGroups() []RepoGroupDef {
	return []RepoGroupDef{
		{
			Name:        "Upstream Rebase Repos",
			Type:        "upstream-rebase",
			Description: "Forks of upstream projects managed by rebasebot. Allow force pushes expected.",
			Repos:       UpstreamRebaseRepos,
		},
		{
			Name:        "OADP-Owned Repos (openshift/)",
			Type:        "oadp-owned-openshift",
			Description: "Should have enforce_admins, review count, and dismiss_stale_reviews.",
			Repos:       OADPOwnedOpenshiftRepos,
		},
		{
			Name:        "OADP-Owned Repos (migtools/)",
			Type:        "oadp-owned-migtools",
			Description: "Should have enforce_admins, review count, and dismiss_stale_reviews.\nNote: migtools repos list plugins explicitly (no org-level inheritance).",
			Repos:       OADPOwnedMigtoolsRepos,
		},
	}
}

// Severity levels for findings.
type Severity int

const (
	SeverityOK Severity = iota
	SeverityInfo
	SeverityWarning
	SeverityIssue
)

func (s Severity) String() string {
	switch s {
	case SeverityOK:
		return "ok"
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityIssue:
		return "issue"
	default:
		return "unknown"
	}
}

// Finding is a single audit result for a repo.
type Finding struct {
	Severity     Severity `json:"severity"`
	Field        string   `json:"field"`
	Message      string   `json:"message"`
	SuggestedFix string   `json:"suggested_fix,omitempty"`
}

// Key returns a string that uniquely identifies this finding for diff comparison.
func (f Finding) Key() string {
	return fmt.Sprintf("%d|%s|%s", f.Severity, f.Field, f.Message)
}

// PluginStatus captures plugin config presence.
type PluginStatus struct {
	HasApproveSection bool `json:"has_approve_section"`
	HasLgtmSection    bool `json:"has_lgtm_section"`
	HasApprovePlugin  bool `json:"has_approve_plugin"`
}

// TideConfig captures tide query details.
type TideConfig struct {
	IncludedBranches         []string `json:"included_branches"`
	RequiredLabels           []string `json:"required_labels"`    // labels required for merge (e.g. approved, lgtm)
	MissingLabels            []string `json:"missing_labels"`     // labels that block merge (e.g. do-not-merge/hold)
	HasKeepMainQuerySeparate bool     `json:"has_keep_main_query_separate"`
}

// OwnersInfo captures OWNERS file presence and contents.
type OwnersInfo struct {
	HasFile   bool     `json:"has_file"`
	FilePath  string   `json:"file_path"`           // "OWNERS" or "DOWNSTREAM_OWNERS"
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// RepoAudit holds the complete audit result for one repository.
type RepoAudit struct {
	Name      string            `json:"name"`
	HasConfig bool              `json:"has_config"`
	Fields    map[string]string `json:"fields"`
	Plugins   PluginStatus      `json:"plugins"`
	Tide      TideConfig        `json:"tide"`
	Owners    OwnersInfo        `json:"owners"`
	Findings  []Finding         `json:"findings"`
}

// WorstSeverity returns the highest severity among findings.
func (r *RepoAudit) WorstSeverity() Severity {
	worst := SeverityOK
	for _, f := range r.Findings {
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return worst
}

// CountBySeverity returns counts of each severity level.
func (r *RepoAudit) CountBySeverity() (issues, warnings, infos int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityIssue:
			issues++
		case SeverityWarning:
			warnings++
		case SeverityInfo:
			infos++
		}
	}
	return
}

// RepoGroup is a group of repos with their audit results.
type RepoGroup struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Repos       []RepoAudit `json:"repos"`
}

// Outlier is a repo whose field value differs from the majority.
type Outlier struct {
	Repo  string `json:"repo"`
	Value string `json:"value"`
}

// FieldComparison shows the majority value and any outliers.
type FieldComparison struct {
	Majority string    `json:"majority"`
	Outliers []Outlier `json:"outliers"`
}

// RateLimit holds GitHub API rate limit info.
type RateLimit struct {
	Remaining        int       `json:"remaining"`
	Limit            int       `json:"limit"`
	ResetAt          time.Time `json:"reset_at"`
	GraphQLRemaining int       `json:"graphql_remaining"`
	GraphQLLimit     int       `json:"graphql_limit"`
	GraphQLResetAt   time.Time `json:"graphql_reset_at"`
}

// Summary holds aggregate counts.
type Summary struct {
	Issues   int `json:"issues"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

// AuditReport is the top-level result of a full audit run.
type AuditReport struct {
	Source     string                                 `json:"source"`
	Timestamp time.Time                              `json:"timestamp"`
	RateLimit *RateLimit                             `json:"rate_limit,omitempty"`
	Groups    []RepoGroup                            `json:"groups"`
	Comparison map[string]map[string]FieldComparison `json:"comparison"`
	Summary   Summary                                `json:"summary"`
}

// --- Merge Queue types ---

// CheckEntry is a single CI check with its name and detail URL.
type CheckEntry struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// CheckStatus holds categorized CI check results for a PR.
type CheckStatus struct {
	Failing    []CheckEntry `json:"failing"`
	Errored    []CheckEntry `json:"errored"`
	Pending    []CheckEntry `json:"pending"`
	InProgress []CheckEntry `json:"in_progress"`
	TideState  string       `json:"tide_state"`
}

// PRStatus holds the analysis of a single PR in the merge queue.
type PRStatus struct {
	Number             int         `json:"number"`
	Title              string      `json:"title"`
	Author             string      `json:"author"`
	Base               string      `json:"base"`
	Head               string      `json:"head"`
	ApprovalCount      int         `json:"approval_count"`
	RequiredReviews    int         `json:"required_reviews"`
	ApprovingReviewers []string    `json:"approving_reviewers"`
	LabelBlockers      []string    `json:"label_blockers"`
	Checks             CheckStatus `json:"checks"`
	HasBlockers        bool        `json:"has_blockers"`
	ReviewBlocked      bool        `json:"review_blocked"`
	ReviewsShort       int         `json:"reviews_short"`
	BranchBlockers     []string    `json:"branch_blockers,omitempty"` // review-blocked PRs ahead on same branch
	PRURL              string      `json:"pr_url"`
	ProwURL            string      `json:"prow_url"`
}

// MergeQueueReport holds merge queue analysis for one repo.
type MergeQueueReport struct {
	Repo            string     `json:"repo"`
	RequiredReviews int        `json:"required_reviews"`
	EnforceAdmins   bool       `json:"enforce_admins"`
	TideURL         string     `json:"tide_url"`
	PRs             []PRStatus `json:"prs"`
	Error           string     `json:"error,omitempty"`
}
