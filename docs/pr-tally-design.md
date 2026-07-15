# PR Tally Tracking Design

Track a running count of rebase PRs opened and merged per OADP branch,
displayed on the wiki Home page and reset when a branch reaches 100% ready.

## Problem

The wiki shows current open PRs but provides no historical context for the
current rebase cycle — how many PRs have been opened and merged since the last
time all repos were aligned. This makes it hard to gauge overall progress.

## Approach: GitHub Search API + Stored Reset Timestamp

Each wiki update queries the GitHub Search API to count PRs opened and merged
since a stored reset point. This is the most accurate method because GitHub is
the authoritative source — it never misses PRs, even those opened and merged
between hourly runs.

### State File

A `pr-tallies.json` stored in the **wiki git repo** (alongside the `.md` pages):

```json
{
  "oadp-1.6": {"reset_at": "2026-07-01T00:00:00Z"},
  "oadp-1.5": {"reset_at": "2026-06-15T00:00:00Z"},
  "oadp-1.4": {"reset_at": "2026-05-20T00:00:00Z"},
  "oadp-1.3": {"reset_at": "2026-04-10T00:00:00Z"}
}
```

Only the reset timestamp is persisted — counts are computed live each run.

### Counting Mechanism

Two GitHub Search API calls per branch:

```
GET /search/issues?q=is:pr+author:oadp-rebasebot+base:{branch}+created:>={reset_at}
→ response.total_count = total PRs opened this cycle (includes already-merged ones)

GET /search/issues?q=is:pr+author:oadp-rebasebot+base:{branch}+merged:>={reset_at}
→ response.total_count = total PRs merged this cycle
```

"Opened" means *ever created* since reset, not *currently open*. A PR that was
opened and merged in the same cycle increments both counters.

The `total_count` field is returned without needing to paginate results.

**API budget:** 2 calls/branch × 4 branches = 8 calls per workflow run.
Well within the Search API rate limit of 30 requests/minute.

### Reset Condition

A branch's counters reset to zero when ALL of the following are true:

- `ready == total` (100% of repos have no error-severity issues)
- `len(CVEPRs) == 0` (no open CVE fix PRs)

Note: `ready == total` already implies zero errors (a repo is "ready" when it
has no error-severity issues), but both conditions are checked explicitly for
clarity.

When triggered, `reset_at` is set to the current UTC time. The current run
still displays the pre-reset counts (the "final score" of the completed cycle).
The *next* run's search queries will return 0 results since no PRs will have
been created/merged after the new reset timestamp.

### Display

Inline with the existing score line on the wiki Home page:

```
3/18 repos ready (16%) | 15 error(s) | 📬 12 opened, 8 merged
```

After a branch completes and resets:

```
18/18 repos ready (100%) | 📬 0 opened, 0 merged
```

The Slack copy snippet includes the same tally.

## Data Flow

```
Workflow starts
    │
    ├─ Clone wiki repo (/tmp/wiki)
    │       └─ Contains pr-tallies.json
    │
    ├─ rebase-status --format home --tally-file /tmp/wiki/pr-tallies.json $BRANCHES
    │       │
    │       ├─ Load pr-tallies.json (read reset timestamps)
    │       ├─ For each branch:
    │       │     ├─ Run all checks (existing logic)
    │       │     ├─ GitHub Search API: count opened since reset_at
    │       │     ├─ GitHub Search API: count merged since reset_at
    │       │     ├─ Check reset condition (100% ready + 0 errors + 0 CVEs)
    │       │     │     └─ If yes: set reset_at = now (counts become 0 next run)
    │       │     └─ Render tallies inline in Home.md
    │       │
    │       └─ Write updated pr-tallies.json
    │
    ├─ git add -A (picks up both .md and .json changes)
    └─ git push
```

## Implementation

### Files to Change

| File | Change |
|------|--------|
| `tools/rebase-status/types.go` | Add `PRTally` struct |
| `tools/rebase-status/github.go` | Add `SearchRebasePRCounts(branch, since)` method |
| `tools/rebase-status/main.go` | Add `--tally-file` flag; load/save JSON; pass tallies to renderer |
| `tools/rebase-status/render_home.go` | Accept tallies; display inline in `homeScoreLine()` |
| `tools/rebase-status/render_home_test.go` | Test tally display and reset logic |
| `.github/workflows/rebase-status-wiki.yaml` | Pass `--tally-file /tmp/wiki/pr-tallies.json` to the `--format home` invocation |

### New Types

```go
// PRTally holds the per-branch tally state.
type PRTally struct {
    ResetAt time.Time `json:"reset_at"`
}

// PRTallyResult holds the computed counts for display.
type PRTallyResult struct {
    Opened  int
    Merged  int
    ResetAt time.Time
}
```

### New GitHub Client Method

```go
// SearchRebasePRCounts returns the number of rebase PRs opened and merged
// for a given base branch since the specified time.
func (c *GitHubClient) SearchRebasePRCounts(branch string, since time.Time) (opened, merged int, err error) {
    sinceStr := since.Format("2006-01-02")

    // Count opened
    openedResp := searchIssues("is:pr author:oadp-rebasebot base:" + branch + " created:>=" + sinceStr)
    opened = openedResp.TotalCount

    // Count merged
    mergedResp := searchIssues("is:pr author:oadp-rebasebot base:" + branch + " merged:>=" + sinceStr)
    merged = mergedResp.TotalCount

    return opened, merged, nil
}
```

### Workflow Change

```yaml
- name: Generate Home page
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    tools/rebase-status/rebase-status \
      --config-dir rebase-configs \
      --format home \
      --tally-file /tmp/wiki/pr-tallies.json \
      $BRANCHES \
      > /tmp/Home.md
```

The `--tally-file` flag is only used with `--format home`. The tally file is
read at the start and written back after all branches are processed, before
the workflow's existing `git add -A && git push` step.

## Alternatives Considered

### Incremental counters in wiki JSON

Store running counters and a list of known PR numbers. Each run diffs current
state vs previous to increment. **Rejected** because PRs opened and merged
between hourly runs would be missed entirely.

### Per-repo pulls API query

Query each repo's closed PRs and filter by merge date client-side.
**Rejected** because it requires 20+ paginated API calls per branch and the
pulls API does not support `merged_at` filtering server-side.

## Edge Cases

- **First run (no tally file):** If the file doesn't exist, initialize
  `reset_at` to the current time. Counts start at 0.
- **Branch not in tally file:** Same as first run — initialize with current
  time.
- **Search API unavailable:** Log a warning, display "—" instead of counts.
  Don't update the tally file for that branch.
- **Bot username variants:** The search query uses `author:oadp-rebasebot`.
  If the GitHub App bot (`oadp-rebasebot-app[bot]`) is also used, a second
  query with that author can be added (or use `author:app/oadp-rebasebot-app`).
