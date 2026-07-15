package main

import "time"

// Status represents the result of a single check.
type Status int

const (
	StatusOK   Status = iota // check passed
	StatusFail               // check failed — action needed
	StatusWarn               // check passed with warning
	StatusSkip               // check was skipped (e.g. SKIP_REPO)
	StatusNA                 // check not applicable for this repo
)

func (s Status) Icon() string {
	switch s {
	case StatusOK:
		return "✅"
	case StatusFail:
		return "❌"
	case StatusWarn:
		return "⚠️ "
	case StatusSkip:
		return "SKIP"
	case StatusNA:
		return "—"
	default:
		return "?"
	}
}

// CheckResult is the outcome of a single check on a single repo.
type CheckResult struct {
	Status  Status
	Summary string // short text for the table cell (e.g. "1.25.0")
	Detail  string // longer explanation for verbose mode
}

// RepoSpec describes a repository to check.
type RepoSpec struct {
	Org       string // e.g. "openshift", "migtools"
	Repo      string // e.g. "velero", "kopia"
	Branch    string // e.g. "oadp-1.6"
	Wave      int
	Skip      bool   // SKIP_REPO=true in config
	HasConfig bool   // whether a rebase config file exists
	NoRebase  bool   // not managed by rebasebot; track-only
	Upstream  string // e.g. "vmware-tanzu/velero:release-1.18" or "" for downstream-only
	// Derived from REBASE_REPO config
	RebasebotRepo   string // e.g. "oadp-rebasebot/velero"
	RebasebotBranch string // e.g. "rebase-bot-oadp-1.6"
}

func (s *RepoSpec) FullName() string {
	return s.Org + "/" + s.Repo
}

// RepoStatus holds all check results for one repository.
type RepoStatus struct {
	Spec     RepoSpec
	Checks   map[string]*CheckResult
	Issues   []Issue
	DepSyncs    []DepSync    // internal dependency sync details
	GoModDrifts []GoModDrift // go.mod dependency drift vs upstream
	Images   []ImageInfo  // container image tag status
	Konflux      *KonfluxInfo    // Konflux build config details
	OpenPR       *OpenPRInfo       // open rebase PR if any
	ImageRefData []*ImageRefEntry // from bundle/image-references
	ArtConfigs   []*ArtBuildConfig // from ocp-build-data (may be multiple per repo)
}

// ImageInfo describes the status of a container image tag on Quay.
type ImageInfo struct {
	Name         string // display name
	Namespace    string // quay namespace
	Repo         string // quay repo
	Tag          string
	Exists       bool
	LastModified time.Time
}

// GoModDrift describes a direct dependency where the downstream go.mod
// has an older version than the upstream source go.mod.
type GoModDrift struct {
	Module            string // e.g. "google.golang.org/grpc"
	DownstreamVersion string // version in downstream go.mod
	UpstreamVersion   string // version in upstream go.mod
}

// DepSync describes the sync state of one internal dependency.
type DepSync struct {
	Module          string // e.g. "github.com/openshift/velero"
	Org             string // resolved GitHub org
	Repo            string // resolved GitHub repo
	HaveHash        string // commit hash found in go.mod or submodule pointer
	HeadHash        string // HEAD commit on the dep's branch
	InSync          bool
	Commits         []CommitInfo // commits between HaveHash and HeadHash (when details requested)
	SubmoduleBranch string       // branch from .gitmodules (empty for go.mod deps)
}

// CommitInfo is a single commit's metadata.
type CommitInfo struct {
	SHA     string
	Message string // first line of commit message
}

// KonfluxInfo describes the Konflux build configuration found in a repo.
type KonfluxInfo struct {
	HasDir        bool   // .konflux/ directory exists
	HasDockerfile bool   // konflux.Dockerfile exists
	BuilderTag    string // e.g. "rhel_9_golang_1.25" from the FROM line
}

// OpenPRInfo describes an open rebase PR found on a repo.
type OpenPRInfo struct {
	Number    int
	URL       string
	CreatedAt time.Time
}

// Issue is a problem found during checking.
type Issue struct {
	Severity string // "error" or "warning"
	Repo     string
	Message  string
}

// ImageRefEntry represents one entry from bundle/image-references.
type ImageRefEntry struct {
	ARTName      string // e.g. "oadp-velero-plugin-for-gcp-rhel9"
	ImageRef     string // e.g. "quay.io/konveyor/velero-plugin-for-gcp:oadp-1.6"
	Namespace    string // e.g. "konveyor"
	QuayRepo     string // e.g. "velero-plugin-for-gcp"
	Tag          string // e.g. "oadp-1.6"
	CommentedOut bool   // true if entry was commented out
}

// ArtBuildConfig represents a parsed ocp-build-data image config.
type ArtBuildConfig struct {
	Filename     string   // e.g. "oadp-velero-plugin-for-gcp.yml"
	Name         string   // e.g. "oadp/oadp-velero-plugin-for-gcp-rhel9"
	Mode         string   // e.g. "" (enabled), "disabled", "wip"
	SourceWeb    string   // e.g. "https://github.com/openshift/velero-plugin-for-gcp"
	SourceURL    string   // e.g. "git@github.com:openshift-priv/velero-plugin-for-gcp.git"
	BranchTarget string   // e.g. "oadp-1.6"
	Dockerfile   string   // e.g. "konflux.Dockerfile"
	Component    string   // e.g. "oadp-velero-plugin-for-gcp-container"
	Dependents   []string // e.g. ["oadp-operator"]
}

// Disabled returns true if the ART config has mode: disabled.
func (a *ArtBuildConfig) Disabled() bool {
	return a.Mode == "disabled"
}

// ReleaseData holds release-level metadata fetched once per branch.
type ReleaseData struct {
	ImageRefs    []ImageRefEntry
	ArtConfigs   []*ArtBuildConfig
	HasImageRefs bool
	HasArtBranch bool
	repoImageRefs map[string][]*ImageRefEntry // "org/repo" -> entries
	repoArtConfigs map[string][]*ArtBuildConfig // "org/repo" -> configs (one per image)
}

// ImageRefsFor returns the image-references entries for a repo.
func (rd *ReleaseData) ImageRefsFor(orgRepo string) []*ImageRefEntry {
	if rd == nil {
		return nil
	}
	return rd.repoImageRefs[orgRepo]
}

// ArtConfigsFor returns all ART build configs for a repo (may be multiple per repo).
func (rd *ReleaseData) ArtConfigsFor(orgRepo string) []*ArtBuildConfig {
	if rd == nil {
		return nil
	}
	return rd.repoArtConfigs[orgRepo]
}

// CVEPRInfo describes an open CVE fix PR on a release branch.
type CVEPRInfo struct {
	Org     string
	Repo    string
	Number  int
	Title   string
	URL     string
	Created time.Time
}

// PRTally holds the per-branch tally state persisted in the wiki repo.
type PRTally struct {
	ResetAt time.Time        `json:"reset_at"`
	History []PRCycleHistory `json:"history,omitempty"`
}

// PRCycleHistory records the final tally of a completed rebase cycle.
type PRCycleHistory struct {
	Date      string `json:"date"`
	Score     string `json:"score"`
	Triggered int    `json:"triggered"`
	Opened    int    `json:"opened"`
	Merged    int    `json:"merged"`
}

// PRTallyResult holds the computed PR counts for display.
type PRTallyResult struct {
	Triggered int
	Opened    int
	Merged    int
	ResetAt   time.Time
}

// BranchResult holds the check results for all repos on a single branch.
type BranchResult struct {
	Branch         string
	Statuses       []RepoStatus
	VeleroTagAlign *VeleroTagAlignment
	CVEPRs         []CVEPRInfo
	Tally          *PRTallyResult
}

// VeleroTagAlignment holds the branch-level velero tag alignment status.
// Computed after all checks run by comparing each repo's pinned velero
// commit against VELERO_TAG_SHA from the versions SSOT.
type VeleroTagAlignment struct {
	VeleroTag    string
	VeleroTagSHA string
	AllAligned   bool
	Repos        []VeleroTagRepo
}

// VeleroTagRepo describes one repo's velero dependency alignment.
type VeleroTagRepo struct {
	Org          string
	Repo         string
	PinnedHash   string
	Aligned      bool
	CompareError string
}

// Check is a registered check that can run against a repo.
// Adding a new check = define a new Check struct and append to the registry.
type Check struct {
	ID     string // unique key, e.g. "branch"
	Header string // column header for table output, e.g. "Branch"
	Run    CheckFunc
}

// CheckFunc is the signature for all check functions.
type CheckFunc func(client *GitHubClient, spec *RepoSpec) *CheckResult

// WaveInfo describes a wave for display.
type WaveInfo struct {
	Number int
	Name   string
}

// JobRun represents a Prow periodic job run.
type JobRun struct {
	Name  string
	State string // SUCCESS, FAILURE, PENDING, etc.
	When  time.Time
	URL   string
}
