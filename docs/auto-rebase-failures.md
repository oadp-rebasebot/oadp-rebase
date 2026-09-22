# Auto-Rebase Failure Runbook

When the [Auto Rebase](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml) badge turns red, follow this process to diagnose and fix the failure.

## Step 1 — Identify the failed job

Click the badge or go to the [Actions tab](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml) and open the failed run. Each failing job is named `rebase (<branch>, <target>, <trigger>)`, e.g. `rebase (oadp-1.6, velero-plugin-for-aws-oadp-1.6, upstream-changed)`. Copy the job URL from the browser.

## Step 2 — Analyze the failure

Use the `/analyze-rebase-failure` skill from [`migtools/oadp-rebase-ai-helpers`](https://github.com/migtools/oadp-rebase-ai-helpers) to classify the failure and get a concrete fix proposal:

```
/analyze-rebase-failure <job-url>
```

The skill fetches the CI logs and classifies the failure into one of these categories:

| Category | Signature | Fix |
|----------|-----------|-----|
| **Cherry-pick conflict** | `WARNING - Upstream content may have been dropped from '<FILE>'` + `ERROR - Manual intervention is needed` | See [Manual rebase](#manual-rebase) below |
| **Hook script failure** | `Unable to run 'go vet'` / `Unable to run 'go mod tidy'` | Fix the failing code or hook; see [Manual rebase](#manual-rebase) if source conflicts caused broken code |
| **Expected divergence** | `'<FILE>' is not covered by any configured hook` — file intentionally differs downstream (OWNERS, CODEOWNERS, repo-specific config) | See [Expected divergence](#expected-divergence-owners-codeowners-repo-specific-files) below |
| **go.mod/go.sum only** | Triage says `safe=true` but retry still fails | Check wave ordering — the dependency may not be merged yet |
| **Infrastructure** | Image pull errors, auth failures, network timeouts | Re-run the failed jobs from the Actions tab |
| **Config or setup** | `Config file not found` / `Missing VAR from versions env` | Fix the config or versions file |

## Step 3 — Apply the fix

### Expected divergence (OWNERS, CODEOWNERS, repo-specific files)

Some files are intentionally managed differently in the downstream fork and will never match the upstream — `OWNERS`, `CODEOWNERS`, `DOWNSTREAM_OWNERS`, repo-specific CI configs, etc. When rebasebot warns that one of these was dropped by a cherry-pick, the conflict is safe: the cherry-pick result is correct by definition.

Add the file to the repo's `expected_conflicts` list in `repos.yaml`:

```yaml
- org: openshift
  repo: velero-plugin-for-aws
  expected_conflicts:
    - OWNERS           # downstream team manages its own contributor list
    - CODEOWNERS       # downstream-only review routing
```

The triage system (`tools/auto-rebase/conflict-triage.sh`) will then treat conflicts in these files as safe and retry with `--conflict-policy warn` automatically — no manual intervention needed on the next run.

Run `make test`, commit, and open a PR against `oadp-dev`.

### Manual rebase

Use this when source code files conflict and the auto-resolution produces broken code (e.g. `go vet` fails after triage retries with `--conflict-policy warn`).

#### Preferred: use the AI skill

The [`migtools/oadp-rebase-ai-helpers`](https://github.com/migtools/oadp-rebase-ai-helpers) plugin provides Claude Code skills that automate rebase workflows. Install it, then run:

```
/oadp-rebase:rebase <target>
```

The skill runs the full rebase pipeline locally, surfaces conflicts, and guides you through resolution. Use `--dry-run` to preview first. If the skill encounters a `go vet` failure it will stop at the working tree so you can inspect and fix the broken code before re-running.

#### Manual fallback (step-by-step)

**1. Temporarily allow warn policy in the config:**

Edit `rebase-configs/<config>.env.sh` for the failing target:

```bash
# Before:
EXTRA_REBASEBOT_ARGS="--always-run-hooks"
# After (temporary):
EXTRA_REBASEBOT_ARGS="--always-run-hooks --conflict-policy warn"
```

**2. Run rebasebot locally:**

```bash
./run-oadp-rebase.sh --local-hooks \
  --working-dir ~/.rebasebot/workdir \
  -s ~/.rebasebot/secrets \
  <target>
```

Rebasebot will cherry-pick all commits and run hooks. It will fail at `go vet` if there are source conflicts — but the working tree at `~/.rebasebot/workdir/<repo>/` is preserved in the state after the cherry-picks and before the failed hook.

**3. Fix the broken code:**

```bash
cd ~/.rebasebot/workdir/<repo>
go vet ./... 2>&1   # see what's wrong
# Edit the conflicted source files to restore dropped content
go vet ./... 2>&1   # verify clean
```

Common pattern: a carry commit was written against an older upstream. The newer upstream added fields or constants in the same area; the cherry-pick dropped them. Restore the upstream additions alongside the carry's own changes. To see what the upstream version of the file looks like, use `git show source/<upstream-tag>:<path/to/file>` in the working directory (rebasebot configures `source`, `dest`, and `rebase` remotes automatically).

**4. Commit the fix:**

```bash
git add <fixed-files>
git commit -m "UPSTREAM: <carry>: Restore <description> dropped by rebase conflict resolution"
```

**5. Run missed hooks and update go modules:**

If rebasebot failed mid-hook-chain, later hooks never ran. Check the CI log for the last `INFO - Running LifecycleHook` line to see where it stopped. At minimum, run `go mod tidy` and commit:

```bash
go mod tidy
git add go.mod go.sum
git commit -m "UPSTREAM: <drop>: Updating go modules"
```

If `normalize-dockerfiles-and-commit.sh` was not reached, check whether any Dockerfiles need it:

```bash
grep -rE 'FROM\s+.*\bgolang:[0-9]+\.[0-9]+\.[0-9]+' --include='Dockerfile*' --include='Containerfile*' .
```

If there are matches, run the hook:

```bash
bash /path/to/oadp-rebase/rebasebot-hook-scripts/normalize-dockerfiles-and-commit.sh
```

**6. Push to the rebase branch and open a PR:**

```bash
git push rebase HEAD:rebase-bot-<branch> --force

gh pr create \
  --repo <org>/<repo> \
  --base <branch> \
  --head oadp-rebasebot:rebase-bot-<branch> \
  --title "Rebase <repo> to <upstream-tag>" \
  --body "Manual rebase. <describe what conflicts were resolved and why.>"
```

**7. Revert the config change:**

```bash
# In rebase-configs/<config>.env.sh, remove --conflict-policy warn:
EXTRA_REBASEBOT_ARGS="--always-run-hooks"
```

Commit and push this revert separately so it does not appear in the rebase PR.
