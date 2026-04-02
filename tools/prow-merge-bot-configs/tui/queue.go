package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// QueueConfig holds the branch-protection settings needed for merge queue analysis.
type QueueConfig struct {
	RequiredReviews int
	EnforceAdmins   bool
}

// blockingLabels are the labels that block merge even with approved+lgtm.
var blockingLabels = []string{
	"do-not-merge/hold",
	"do-not-merge/work-in-progress",
	"do-not-merge/invalid-owners-file",
	"needs-rebase",
	"jira/invalid-bug",
	"backports/unvalidated-commits",
}

// --- Internal structs for unmarshalling gh CLI JSON output ---

type ghPR struct {
	Number            int               `json:"number"`
	HeadRefName       string            `json:"headRefName"`
	BaseRefName       string            `json:"baseRefName"`
	Author            ghAuthor          `json:"author"`
	Title             string            `json:"title"`
	Labels            []ghLabel         `json:"labels"`
	StatusCheckRollup []json.RawMessage `json:"statusCheckRollup"`
	Reviews           []ghReview        `json:"reviews"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghReview struct {
	Author      ghAuthor `json:"author"`
	State       string   `json:"state"`
	SubmittedAt string   `json:"submittedAt"`
}

// ghCheckRun represents a CheckRun from statusCheckRollup.
// CheckRuns have "name", "status", and "conclusion" fields.
type ghCheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	DetailsURL string `json:"detailsUrl"`
}

// ghStatusContext represents a StatusContext from statusCheckRollup.
// StatusContexts have "context", "state", and "targetUrl" fields.
type ghStatusContext struct {
	Context   string `json:"context"`
	State     string `json:"state"`
	TargetURL string `json:"targetUrl"`
}

// normalizedCheck is a unified representation of a CI check.
type normalizedCheck struct {
	Name  string
	State string // SUCCESS, FAILURE, ERROR, IN_PROGRESS, PENDING
	URL   string // direct link to the check run/status details
}

// normalizeChecks converts the mixed statusCheckRollup entries into
// a uniform list of {name, state} pairs.
func normalizeChecks(raw []json.RawMessage) []normalizedCheck {
	var checks []normalizedCheck
	for _, entry := range raw {
		// Try to determine the type by looking for distinguishing fields.
		// StatusContext objects have "context"; CheckRun objects have "name".
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(entry, &probe); err != nil {
			continue
		}

		if _, hasContext := probe["context"]; hasContext {
			// StatusContext
			var sc ghStatusContext
			if err := json.Unmarshal(entry, &sc); err != nil {
				continue
			}
			state := normalizeStatusContextState(sc.State)
			checks = append(checks, normalizedCheck{Name: sc.Context, State: state, URL: sc.TargetURL})
		} else {
			// CheckRun
			var cr ghCheckRun
			if err := json.Unmarshal(entry, &cr); err != nil {
				continue
			}
			state := normalizeCheckRunState(cr.Status, cr.Conclusion)
			checks = append(checks, normalizedCheck{Name: cr.Name, State: state, URL: cr.DetailsURL})
		}
	}
	return checks
}

// normalizeCheckRunState maps CheckRun status+conclusion to a unified state.
func normalizeCheckRunState(status, conclusion string) string {
	switch strings.ToUpper(status) {
	case "COMPLETED":
		if conclusion == "" {
			return "UNKNOWN"
		}
		return strings.ToUpper(conclusion)
	case "IN_PROGRESS":
		return "IN_PROGRESS"
	case "QUEUED":
		return "PENDING"
	default:
		if status != "" {
			return strings.ToUpper(status)
		}
		return "PENDING"
	}
}

// normalizeStatusContextState maps StatusContext state to a unified state.
func normalizeStatusContextState(state string) string {
	upper := strings.ToUpper(state)
	switch upper {
	case "SUCCESS", "FAILURE", "ERROR":
		return upper
	default:
		return "PENDING"
	}
}

// countApprovals groups reviews by author, keeps the latest per author,
// and returns the count of APPROVED reviews along with the approving logins.
func countApprovals(reviews []ghReview) (int, []string) {
	if len(reviews) == 0 {
		return 0, nil
	}

	// Group by author login, keep all reviews per author.
	byAuthor := make(map[string][]ghReview)
	for _, r := range reviews {
		login := r.Author.Login
		byAuthor[login] = append(byAuthor[login], r)
	}

	var approvers []string
	for login, authorReviews := range byAuthor {
		// Sort by submittedAt ascending, take the last one.
		sort.Slice(authorReviews, func(i, j int) bool {
			return authorReviews[i].SubmittedAt < authorReviews[j].SubmittedAt
		})
		latest := authorReviews[len(authorReviews)-1]
		if strings.ToUpper(latest.State) == "APPROVED" {
			approvers = append(approvers, login)
		}
	}

	// Sort approvers for deterministic output.
	sort.Strings(approvers)
	return len(approvers), approvers
}

// detectBlockingLabels returns the subset of labels that block merge.
func detectBlockingLabels(labels []ghLabel) []string {
	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l.Name] = true
	}

	var blockers []string
	for _, bl := range blockingLabels {
		if labelSet[bl] {
			blockers = append(blockers, bl)
		}
	}
	return blockers
}

// categorizeChecks separates normalized checks into failing, errored, pending,
// in_progress lists and extracts the tide state. The "tide" check is excluded
// from all lists except tide_state.
func categorizeChecks(checks []normalizedCheck) CheckStatus {
	var cs CheckStatus
	for _, c := range checks {
		if strings.EqualFold(c.Name, "tide") {
			cs.TideState = c.State
			continue
		}
		entry := CheckEntry{Name: c.Name, URL: c.URL}
		switch c.State {
		case "FAILURE":
			cs.Failing = append(cs.Failing, entry)
		case "ERROR":
			cs.Errored = append(cs.Errored, entry)
		case "PENDING":
			cs.Pending = append(cs.Pending, entry)
		case "IN_PROGRESS":
			cs.InProgress = append(cs.InProgress, entry)
		}
	}
	// Sort each list by name for deterministic output.
	sortChecks := func(entries []CheckEntry) {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name < entries[j].Name
		})
	}
	sortChecks(cs.Failing)
	sortChecks(cs.Errored)
	sortChecks(cs.Pending)
	sortChecks(cs.InProgress)

	if cs.TideState == "" {
		cs.TideState = "NOT_REPORTED"
	}
	return cs
}

// splitRepo splits "org/reponame" into its two parts.
func splitRepo(repo string) (org, reponame string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return repo, ""
}

// CheckMergeQueue fetches open PRs with approved+lgtm labels for a repo
// and analyzes what is blocking each one from merging.
func CheckMergeQueue(repo string, qc *QueueConfig) (*MergeQueueReport, error) {
	// Verify gh CLI is available.
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not found: %w", err)
	}

	org, reponame := splitRepo(repo)

	// Fetch open PRs with approved+lgtm labels.
	cmd := exec.Command("gh", "pr", "list",
		"--repo", repo,
		"--state", "open",
		"--label", "approved",
		"--label", "lgtm",
		"--json", "number,headRefName,baseRefName,author,title,labels,statusCheckRollup,reviews",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list failed for %s: %w", repo, err)
	}

	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parsing gh pr list JSON for %s: %w", repo, err)
	}

	// Get config values.
	var requiredReviews int
	var enforceAdmins bool
	if qc != nil {
		requiredReviews = qc.RequiredReviews
		enforceAdmins = qc.EnforceAdmins
	}

	// Build Prow Tide URL.
	tideURL := fmt.Sprintf("https://prow.ci.openshift.org/tide?query=is%%3Apr+state%%3Aopen+repo%%3A%s%%2F%s",
		url.PathEscape(org), url.PathEscape(reponame))

	report := &MergeQueueReport{
		Repo:            repo,
		RequiredReviews: requiredReviews,
		EnforceAdmins:   enforceAdmins,
		TideURL:         tideURL,
		PRs:             make([]PRStatus, 0, len(prs)),
	}

	// Pre-scan: identify review-blocked PRs that will block the queue.
	// A PR is a queue blocker if enforce_admins=true, approvals < required, AND tide=SUCCESS.
	type reviewBlocker struct {
		base        string
		number      int
		reviewShort int
	}
	var reviewBlockers []reviewBlocker
	if enforceAdmins && requiredReviews > 0 {
		for _, pr := range prs {
			approvals, _ := countApprovals(pr.Reviews)
			if approvals < requiredReviews {
				normalized := normalizeChecks(pr.StatusCheckRollup)
				checks := categorizeChecks(normalized)
				if checks.TideState == "SUCCESS" || checks.TideState == "EXPECTED" {
					reviewBlockers = append(reviewBlockers, reviewBlocker{
						base:        pr.BaseRefName,
						number:      pr.Number,
						reviewShort: requiredReviews - approvals,
					})
				}
			}
		}
	}

	for _, pr := range prs {
		// Normalize checks.
		normalized := normalizeChecks(pr.StatusCheckRollup)
		checks := categorizeChecks(normalized)

		// Count approvals.
		approvalCount, approvers := countApprovals(pr.Reviews)

		// Detect blocking labels.
		labelBlockers := detectBlockingLabels(pr.Labels)

		// Detect review shortfall.
		reviewBlocked := false
		reviewsShort := 0
		if enforceAdmins && requiredReviews > 0 && approvalCount < requiredReviews {
			reviewBlocked = true
			reviewsShort = requiredReviews - approvalCount
		}

		// Tide state: any non-SUCCESS state blocks the PR from merging.
		tideBlocked := checks.TideState != "SUCCESS" && checks.TideState != "NOT_REPORTED"

		// Determine if there are any blockers.
		hasBlockers := len(labelBlockers) > 0 ||
			len(checks.Failing) > 0 ||
			len(checks.Errored) > 0 ||
			len(checks.Pending) > 0 ||
			len(checks.InProgress) > 0 ||
			reviewBlocked ||
			tideBlocked

		// Build PR URL.
		prURL := fmt.Sprintf("https://github.com/%s/pull/%d", repo, pr.Number)

		// Build Prow PR URL with encoded branch.
		encodedBranch := url.QueryEscape(pr.HeadRefName)
		prowURL := fmt.Sprintf("https://prow.ci.openshift.org/pr?query=is%%3Apr+repo%%3A%s%%2F%s+author%%3A%s+head%%3A%s",
			url.PathEscape(org), url.PathEscape(reponame),
			url.PathEscape(pr.Author.Login), encodedBranch)

		// Find review-blocked PRs ahead on the same branch.
		var branchBlockers []string
		for _, rb := range reviewBlockers {
			if rb.base == pr.BaseRefName && rb.number != pr.Number {
				branchBlockers = append(branchBlockers, fmt.Sprintf("#%d (needs %d more review(s))", rb.number, rb.reviewShort))
			}
		}

		status := PRStatus{
			Number:             pr.Number,
			Title:              pr.Title,
			Author:             pr.Author.Login,
			Base:               pr.BaseRefName,
			Head:               pr.HeadRefName,
			ApprovalCount:      approvalCount,
			RequiredReviews:    requiredReviews,
			ApprovingReviewers: approvers,
			LabelBlockers:      labelBlockers,
			Checks:             checks,
			HasBlockers:        hasBlockers,
			ReviewBlocked:      reviewBlocked,
			ReviewsShort:       reviewsShort,
			BranchBlockers:     branchBlockers,
			PRURL:              prURL,
			ProwURL:            prowURL,
		}

		report.PRs = append(report.PRs, status)
	}

	return report, nil
}

// CheckAllMergeQueues runs CheckMergeQueue for all repos concurrently,
// limited to 5 goroutines at a time. Returns a map of repo to report.
// Repos that fail are included with a report containing the Error field.
func CheckAllMergeQueues(repos []string, configs map[string]*QueueConfig) map[string]*MergeQueueReport {
	results := make(map[string]*MergeQueueReport, len(repos))
	var mu sync.Mutex
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var qc *QueueConfig
			if configs != nil {
				qc = configs[r]
			}

			report, err := CheckMergeQueue(r, qc)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[r] = &MergeQueueReport{Repo: r, Error: err.Error()}
			} else {
				results[r] = report
			}
		}(repo)
	}

	wg.Wait()
	return results
}
