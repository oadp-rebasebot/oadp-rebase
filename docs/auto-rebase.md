# Auto-Rebase Workflow

Runs weekdays at 07:00 UTC (or on manual dispatch) to keep downstream OADP forks in sync with their upstreams.

## Jobs

**decide** — Builds the `rebase-status` tool, runs it against each configured branch, and pipes the JSON output through `rebase-decision.sh` to select which repos need a rebase. Produces a GitHub Actions matrix of `{branch, target, reason}` entries.

**rebase** — Fans out across the matrix. For each target, runs `run-rebase-pipeline.sh`, which orchestrates:

1. **Dry-run** with strict conflict policy
2. **Conflict triage** — if the dry-run fails, checks whether all conflicting files are covered by post-rebase hooks. If safe, retries with `--conflict-policy warn`.
3. **Verify dry-run** — checks that all carry commits and downstream-only files survived the rebase
4. **Real run** — pushes the rebase branch and opens/updates a PR (skipped in dry-run mode)
5. **Post-push verify** — re-runs verification against the pushed branch

Results are reported via workflow annotations, job summaries, and uploaded as artifacts.

## Decision Logic

Wave 1 repos (no internal OADP dependencies) are always eligible. Wave 2+ repos only rebase when their dependencies have moved ahead. Repos with an open rebase PR are skipped. This makes the system self-sequencing across waves.

## Key Files

- `rebase-configs/*.env.sh` — per-repo-per-branch config (upstream, downstream, hooks)
- `run-oadp-rebase.sh` — target-to-config mapping and wave definitions
- `tools/auto-rebase/run-rebase-pipeline.sh` — pipeline orchestration
- `tools/auto-rebase/rebase-decision.sh` — target selection logic
- `tools/auto-rebase/conflict-triage.sh` — conflict safety check
- `tools/verify-rebase.sh` — carry commit and downstream file verification
- `tools/rebase-status/` — Go tool for querying repo state
