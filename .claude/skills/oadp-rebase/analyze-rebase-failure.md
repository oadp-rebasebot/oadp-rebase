---
name: analyze-rebase-failure
description: Fetch and analyze GitHub Actions rebase failures for OADP repos. Use when a rebase CI run fails and you need to understand what broke, why, and how to fix it.
---

<objective>
Analyze a failed Auto Rebase GitHub Actions run on the oadp-rebasebot/oadp-rebase repository. Fetch the CI logs, classify the failure, identify the root cause, and propose concrete fixes (file edits, config changes, or hook script updates).
</objective>

<quick_start>
Invoke with a GitHub Actions run URL, a run ID, or a repo-branch target name.

Examples:
- `/analyze-rebase-failure https://github.com/oadp-rebasebot/oadp-rebase/actions/runs/12345678`
- `/analyze-rebase-failure 12345678`
- `/analyze-rebase-failure velero-oadp-1.6` (finds the most recent failed run for this target)
</quick_start>

<process>

<step_1>
**Identify the failed run**

The upstream repo is `oadp-rebasebot/oadp-rebase`. The workflow is `auto-rebase-v2.yaml`.

If given a **full URL**, extract the run ID:
```
https://github.com/oadp-rebasebot/oadp-rebase/actions/runs/<RUN_ID>
```

If given a **bare run ID**, use it directly.

If given a **target name** (e.g. `velero-oadp-1.6`), find the most recent failed run:
```bash
gh run list --repo oadp-rebasebot/oadp-rebase --workflow auto-rebase-v2.yaml \
  --status failure --limit 10 --json databaseId,displayTitle,startedAt,conclusion
```
Note: `displayTitle` is always generic ("Auto Rebase") and cannot identify which targets were in the run. Instead, inspect the job matrix of candidate runs to find the one that included the given target:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --json jobs --jq '.jobs[].name'
```
Job names follow the pattern `rebase (<branch>, <target>, <trigger>)`, e.g. `rebase (oadp-dev, velero-oadp-dev, deps-changed)`. Check each candidate run until you find one whose job names include the target. If ambiguous, show the user the list and ask which run to analyze.

Verify the run exists and get its metadata:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --json jobs,status,conclusion,startedAt
```
The `jobs` field is the primary way to determine which branch and targets were included in the run, since `displayTitle` is always generic ("Auto Rebase"). Each job name encodes `(branch, target, trigger)` -- check which jobs failed to focus the analysis.
</step_1>

<step_2>
**Fetch the logs**

Get the full log output for failed jobs:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --log-failed 2>/dev/null
```

If `--log-failed` returns nothing useful (some failures happen in the `decide` job), fetch all logs:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --log 2>/dev/null
```

The logs are large. Pipe through `grep` to find the relevant sections:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --log-failed 2>/dev/null | \
  grep -E '(ERROR|WARNING|FAIL|fatal|panic|Unable to|exit code|Manual intervention|conflict|could not resolve|cannot find|unresolvable|image pull|denied|Triage|Phase|hook)' | \
  head -200
```

Extract PR URLs from the logs to include in the summary:
```bash
gh run view <RUN_ID> --repo oadp-rebasebot/oadp-rebase --log 2>/dev/null | \
  grep -E '(I created a new rebase PR|I updated existing rebase PR|PR .* already contains the rebase)' | \
  head -20
```
The three patterns indicate: a new PR was opened, an existing PR was force-pushed, or the PR was already up-to-date. If none match, the run failed before reaching the PR stage.

Also download the result artifact if available:
```bash
gh run download <RUN_ID> --repo oadp-rebasebot/oadp-rebase --pattern 'result-*' --dir /tmp/rebase-results 2>/dev/null
```
If a result JSON exists, read it for structured data (exit codes, PR URLs, triage verdicts).
</step_2>

<step_3>
**Classify the failure**

Analyze the log output to determine the failure category. Work through these in order -- the first match is the primary category.

<category name="cherry-pick-conflict">
**Cherry-pick conflict**

Signature lines in log:
```
WARNING - Upstream content may have been dropped from '<FILENAME>' by cherry-pick
ERROR - Manual intervention is needed to rebase <source> into <dest>
```

Extract from the log:
1. Which commit(s) triggered the conflict (the commit SHA and message after "cherry-pick of:")
2. Which file(s) were affected (the filenames in single quotes after "dropped from")
3. The lost lines listed after each WARNING

Then check whether the conflict-triage system already classified this:
- If the result JSON has `is_manual: true`, triage ran and determined it needs human review.
- If the result JSON has `dry_run_policy: "warn"`, triage determined the conflicts were safe to auto-resolve and the failure happened later.

Determine if the conflict is **hook-coverable** by checking the file against the rule table in `tools/auto-rebase/conflict-triage.sh`:
- `go.mod`, `go.sum` -- covered by `go-mod-tidy-and-commit.sh`
- `Dockerfile`, `Dockerfile-Windows`, `hack/build-image/Dockerfile` -- covered by `normalize-dockerfiles-and-commit.sh`
- Everything else -- needs human review

Read the affected config file to check which hooks are configured:
```bash
cat rebase-configs/<CONFIG_NAME>.env.sh
```
</category>

<category name="hook-script-failure">
**Hook script failure**

Signature lines in log:
```
Unable to run 'go mod tidy' in <path>
Unable to run 'go mod vendor' in <path>
Unable to run 'go vet' in <path>
exit status 1
```

Or any error occurring AFTER "Performing rebase" succeeds (INFO lines for all cherry-picks complete without ERROR).

Identify which hook failed from the log context. Hooks run in the order specified in the config's `HOOK_SCRIPTS` variable. The execution sequence is:
1. Pre-rebase hooks (e.g., `verify-tag-sha_*.sh`)
2. Cherry-picking of downstream commits
3. Post-rebase hooks in order:
   - `fix-malformed-filenames-and-commit.sh`
   - `go-mod-reconcile-upstream.sh`
   - `go-replace_velero_*.sh` / `go-replace_kopia_*.sh`
   - `go-use-tag_*.sh`
   - `go-mod-tidy-and-commit.sh` (this is where most go.mod failures surface)
   - Submodule hooks (`restic-submodule-and-commit_*.sh`, etc.)
   - `normalize-dockerfiles-and-commit.sh`
   - `oadp-operator-copy-crds-from-velero-and-commit_*.sh`

Read the source of the failing hook script to understand what it does:
```bash
cat rebasebot-hook-scripts/<hook-name>.sh
```

Common hook failures:
- **go-mod-tidy-and-commit.sh**: `go mod tidy` fails because a replace directive points to a branch that does not exist yet, or a module cannot be resolved. Look for "cannot find module providing package" or "go: <module>@<version>: reading ... 404 Not Found".
- **go-replace_velero_*.sh**: The `go mod edit -require` fails because the upstream tag format changed.
- **verify-tag-sha_*.sh**: "TAG SHA MISMATCH" means the upstream tag was recreated. The expected SHA in the hook needs updating via `versions/oadp-1.X.env` + `make generate`.
- **submodule hooks**: `git submodule update` fails, usually because the downstream branch does not exist in the submodule repo.
- **oadp-operator-copy-crds-from-velero-and-commit_*.sh**: GitHub API rate limit or the velero fork's branch does not have the expected CRD directory structure.
</category>

<category name="go-module-issue">
**Go module resolution issue**

Signature lines in log:
```
go: <module>@<version>: reading https://proxy.golang.org/...: 404 Not Found
cannot find module providing package <import-path>
go: errors parsing go.mod
module <path> found, but does not contain package <path>
checksum mismatch
```

These typically occur during `go mod tidy` inside `go-mod-tidy-and-commit.sh`, but the root cause is usually a missing or incorrect replace directive from an earlier hook.

Check:
1. Does the go-replace hook for the right version exist and point to the correct downstream branch?
   ```bash
   cat rebasebot-hook-scripts/go-replace_velero_<branch>.sh
   cat rebasebot-hook-scripts/go-replace_kopia_<branch>.sh
   ```
2. Has the downstream dependency actually been rebased and merged? Check wave ordering -- if this repo is wave 3+ and depends on a wave 1-2 repo that has not been rebased yet, the dependency branch may not exist.
3. Is a new upstream dependency being pulled in that needs a new replace directive?

Read the versions SSOT:
```bash
cat versions/<oadp-version>.env
```
</category>

<category name="verify-failure">
**Post-rebase verification failure**

Signature lines in log:
```
Post-push verification failed
Missing carry commits
```

This means the rebase itself succeeded, but `tools/verify-rebase.sh` detected that some downstream carry commits were lost. Read the verify output carefully to see which commits are missing.

Check:
1. Were the missing commits intentionally dropped (commit messages containing `<drop>`)? Those are expected to be absent.
2. Did a cherry-pick silently produce an empty commit? This happens when upstream incorporated the downstream change.
3. Is there a merge commit in the downstream history that confuses the verification logic?
</category>

<category name="infrastructure-failure">
**Infrastructure / environment failure**

Signature lines in log:
```
Error: copying system image from manifest list
unexpected EOF
dial tcp: lookup <host>
denied: access forbidden
GITHUB_TOKEN
rate limit
clock drift
```

These are transient issues, not rebase logic problems.

Common causes:
- **Image pull failure**: The rebasebot container image (`quay.io/migtools/rebasebot:latest`) could not be pulled. Retry.
- **GitHub API rate limit**: Too many API calls. Wait and retry.
- **Auth failure**: GitHub App keys expired or have wrong permissions. Check secrets configuration.
- **Network timeout**: Transient. Retry.
- **Clock drift**: Container clock is off, causing TLS certificate errors.

For infrastructure failures, the solution is almost always to re-run the workflow.
</category>

<category name="config-or-setup-failure">
**Configuration or setup failure**

Signature lines in log:
```
Config file not found
Unknown config
Missing <VAR> from versions env
yq: Error
Could not resolve config for target
```

This means the rebase target's configuration is broken:
- A config file is missing from `rebase-configs/`
- A versions variable referenced in the config is not defined in `versions/oadp-1.X.env`
- The `repos.yaml` entry does not match the config file naming convention
- `yq` could not parse `repos.yaml`

Read the config and versions files to identify the gap.
</category>

</step_3>

<step_4>
**Propose concrete fixes**

Based on the failure category, propose specific changes. Always reference exact file paths and show the proposed edits.

<fix_for category="cherry-pick-conflict">
**If conflicts are in hook-coverable files only** (go.mod, go.sum, Dockerfiles):
- Verify the config has the right hooks. If a hook is missing, show the config edit to add it.
- Suggest re-running with `--conflict-policy warn` if the CI pipeline did not already try that.

**If conflicts are in source code files** (.go, .yaml, etc.):
- The downstream carry commit needs to be manually rebased. Show which commit and files need attention.
- Suggest using `/oadp-rebase:rebase <target>`
- If the conflict is because upstream incorporated the downstream change, suggest dropping the carry commit by renaming it from `<carry>` to `<drop>` in the downstream repo.
</fix_for>

<fix_for category="hook-script-failure">
- If `verify-tag-sha` failed with SHA mismatch: Update `versions/oadp-1.X.env` with the new SHA, then `make generate`.
- If `go mod tidy` failed due to missing module: Check if the dependency's downstream fork has been rebased. If not, that wave must be completed first. If the module genuinely moved (e.g., `vmware-tanzu/velero` to `velero-io/velero`), update the go-replace hook.
- If a submodule hook failed: Verify the downstream branch exists in the submodule repo.
- If `go vet` failed: Check if `GO_VET_SKIP=1` should be set in the config's `EXTRA_REBASEBOT_ARGS`, or fix the actual vet error.
- Show the exact file edit needed.
</fix_for>

<fix_for category="go-module-issue">
- If a replace directive is missing: Show the edit to the go-replace hook or suggest creating a new hook.
- If a module 404s on the proxy: Check if `GOPRIVATE` in `go-mod-tidy-and-commit.sh` needs to include the module's domain.
- If the wave dependency is not merged yet: Explain the wave ordering and suggest waiting or rebasing the dependency first.
- Show exact go.mod edits or hook script changes.
</fix_for>

<fix_for category="verify-failure">
- List the missing carry commits with their SHA and subject.
- For each, determine if it was intentionally dropped or accidentally lost.
- If accidentally lost, re-run with `/oadp-rebase:rebase <target> --branch <branch>`. Check if a conflict resolution silently emptied the commit.
</fix_for>

<fix_for category="infrastructure-failure">
- Suggest re-running the workflow: "Go to the Actions tab and click 'Re-run failed jobs'."
- If auth-related, suggest checking the GitHub App key expiry and permissions.
- If image-pull related, suggest checking quay.io/migtools/rebasebot status.
</fix_for>

<fix_for category="config-or-setup-failure">
- Show the exact config file or versions file that needs to be created or fixed.
- If a new repo needs configuration, reference AGENTS.md for the setup procedure.
- If repos.yaml is wrong, show the corrected entry.
</fix_for>

</step_4>

<step_5>
**Reproduce locally when logs are insufficient**

CI logs often show WHAT failed but not WHY. When the root cause isn't clear from logs alone (e.g., a hook failed but the error is truncated, or a go.mod issue needs investigation), reproduce the failure locally using the rebase skill:

```
/oadp-rebase:rebase <target> --branch <branch> --dry-run
```

This runs the same rebasebot + hooks pipeline locally with full output, letting you:
- See the complete cherry-pick diff and conflict markers
- Inspect the working tree mid-rebase (go.mod state, missing files, replace directives)
- Test hook fixes by editing hook scripts and re-running
- Trace exactly which hook or cherry-pick step produces the failure

Use the local reproduction to validate your diagnosis before proposing fixes. The pattern from a real session: CI showed `go mod tidy` failing on `pkg/apis@v0.0.0`, but only by inspecting the working tree's go.mod locally could we see that the upstream `replace` directive was dropped during cherry-pick.
</step_5>

<step_6>
**Present findings**

Format the analysis as:

```
## Rebase Failure Analysis: `<target>` (<branch>)

**Run:** [#<run_number>](<run_url>)
**Category:** <category name>
**Failed at:** <phase (dry-run strict / triage / dry-run warn / verify / real run)>
**PR:** [<pr-url>](<pr-url>) (new | updated | already up-to-date) — or "No PR opened" if the run failed before reaching the PR stage

### What happened

<Clear explanation of the failure, citing specific log lines>

### Root cause

<Why this happened -- what changed upstream, what dependency is missing, etc.>

### Proposed fix

<Specific file edits with paths, or commands to run>

### Next steps

1. <Apply the fix>
2. <Run `make test` to validate>
3. <Commit and push>
4. Re-run the rebase with `/oadp-rebase:rebase <target> --branch <branch>`
```

If multiple targets failed in the same run, analyze each one separately.
</step_6>

</process>

<reference_files>
Files to read when analyzing failures -- these contain the logic that produces the log output:

| File | Purpose |
|------|---------|
| `tools/auto-rebase/run-rebase-pipeline.sh` | Pipeline phases: dry-run, triage, retry, verify, real-run |
| `tools/auto-rebase/conflict-triage.sh` | Determines if cherry-pick conflicts are hook-coverable |
| `tools/auto-rebase/resolve-config.sh` | Maps target name to config file and repo metadata |
| `tools/verify-rebase.sh` | Post-rebase carry commit verification |
| `run-oadp-rebase.sh` | Config loading, hook transformation, rebasebot invocation |
| `repos.yaml` | SSOT for repo definitions, waves, config prefixes |
| `versions/oadp-1.X.env` | SSOT for upstream tags and SHAs |
| `rebase-configs/<config>.env.sh` | Per-repo config with hook list |
| `rebasebot-hook-scripts/*.sh` | Individual hook scripts |
| `.github/workflows/auto-rebase-v2.yaml` | CI workflow definition |
</reference_files>

<rebasebot_log_format>
Rebasebot produces structured log lines. Key patterns:

```
INFO - Using working directory: /tmp/.cache/rebasebot
INFO - Destination repository is https://github.com/<org>/<repo>.git
INFO - rebase repository is https://github.com/oadp-rebasebot/<repo>.git
INFO - source repository is https://github.com/<upstream-org>/<repo>.git
INFO - Fetching <branch> from dest
INFO - Fetching <tag> from source
INFO - Checking out source/<tag>
INFO - Performing rebase
INFO - Picking commit: <sha> - <subject>
WARNING - Upstream content may have been dropped from '<file>' by cherry-pick of: <sha> - <subject>
WARNING -   lost line: <line content>
WARNING -   ... and N more lines
ERROR - Manual intervention is needed to rebase <source> into <dest>
INFO - Running hook: <hook-path>
INFO - I created a new rebase PR: <pr-url>
INFO - I updated existing rebase PR: <pr-url>
INFO - PR <pr-url> already contains the rebase
```

Pipeline wrapper adds phase markers:
```
::group::Phase 1: Dry-run (strict)
::group::Triage
::group::Phase 1b: Dry-run (warn)
::group::Phase 2: Verify dry-run
::group::Phase 3: Real run
```
</rebasebot_log_format>

<success_criteria>
- Failed run identified and logs fetched
- Failure classified into one of the defined categories
- Root cause identified with specific log evidence
- Concrete fix proposed with exact file paths and edits
- Next steps include validation (`make test`) and re-running via `/oadp-rebase:rebase`
</success_criteria>

<see_also>
- `/oadp-rebase:rebase` — Run a rebase (from `migtools/oadp-rebase-ai-helpers` plugin). Use with `--dry-run` to reproduce failures locally.
- `/oadp-rebase:manual-rebase` — Handle repos with cherry-pick conflicts requiring manual intervention.
- `/oadp-rebase:verify-commits` — Verify downstream carry commits are preserved after rebase.
- `/oadp-rebase:status` — Check current rebase status across all repos.
</see_also>
