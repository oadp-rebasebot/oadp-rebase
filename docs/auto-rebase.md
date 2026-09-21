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

## Version Data (SSOT)

All upstream tags and Velero SHAs are stored in `versions/oadp-1.X.env` files — the single source of truth for each OADP release. Configs and hook scripts are derived from these files, never edited directly with version-specific values.

```
versions/oadp-1.3.env    ─┐
versions/oadp-1.4.env    ─┤
versions/oadp-1.5.env    ─┤  make generate
versions/oadp-1.6.env    ─┤  ────────────→  rebasebot-hook-scripts/verify-tag-sha_*.sh
versions/oadp-dev.env    ─┘                 docs/version-matrix.md
```

Each versions file contains:

```bash
OADP_BRANCH="oadp-1.6"
VELERO_UPSTREAM_TAG="v1.18.2-rc.2"
VELERO_TAG_SHA="c253c7fe37d78c9b7e55c68544f7c5b2608712d8"
KOPIA_UPSTREAM_TAG="v0.22.3-velero-patch"
AWS_PLUGIN_TAG="v1.14.1"
GCP_PLUGIN_TAG="v1.14.1"
AZURE_PLUGIN_TAG="v1.14.1"
KUBEVIRT_PLUGIN_REF="release-v0.9"
```

Rebase configs reference these variables (e.g., `UPSTREAM_VELERO_BRANCH="${VELERO_UPSTREAM_TAG:?...}"`), and `load_config()` in `run-oadp-rebase.sh` sources the versions file before each config.

## How to Make Changes

### Bump an upstream tag

1. Edit the versions file (e.g., `versions/oadp-1.6.env`):
   ```bash
   VELERO_UPSTREAM_TAG="v1.18.3"
   VELERO_TAG_SHA="<new-sha>"  # git ls-remote https://github.com/velero-io/velero v1.18.3
   ```
2. Run `make generate` to regenerate verify-tag-sha hooks and docs
3. Run `make test` to verify everything loads correctly
4. Commit all changes

### Add a new OADP version

1. Create `versions/oadp-1.X.env` with all upstream tags
2. Create rebase configs in `rebase-configs/` referencing the SSOT variables
3. Create version-specific hook scripts in `rebasebot-hook-scripts/` (copy from the nearest version and adjust branch names)
4. Add config name mappings and wave definitions in `run-oadp-rebase.sh`
5. Run `make generate && make test`

### Add a new repository to an existing version

1. Add the config file in `rebase-configs/`
2. Add the config mapping in `get_config_name()` in `run-oadp-rebase.sh`
3. Add the repo to the correct wave in `get_wave_repos()` in `run-oadp-rebase.sh`
4. Run `make test` to verify

### Test locally

```bash
make test                              # run all checks
make generate                          # regenerate from SSOT
./run-oadp-rebase.sh -t velero-oadp-1.6  # test single config
./run-oadp-rebase.sh -t -b oadp-1.6 -w 3  # test wave (not yet supported, use individual repos)
./run-oadp-rebase.sh --dry-run --local-hooks --working-dir ~/rebase-workdir -s ~/.rebasebot/secrets velero-oadp-1.6  # dry-run with rebasebot
```

## Key Files

| File | Purpose |
|------|---------|
| `versions/*.env` | SSOT for upstream tags and Velero SHAs |
| `rebase-configs/*.env.sh` | Per-repo-per-branch config (upstream, downstream, hooks) |
| `run-oadp-rebase.sh` | Target-to-config mapping, wave definitions, config loading |
| `rebasebot-hook-scripts/*.sh` | Post-rebase hooks (go-replace, go-mod-tidy, submodules, etc.) |
| `Makefile` | `generate`, `verify-generate`, `test` targets |
| `tools/generate-verify-tag-sha.sh` | Generates verify-tag-sha hooks from SSOT |
| `tools/generate-version-matrix.sh` | Generates docs/version-matrix.md from SSOT |
| `tools/auto-rebase/run-rebase-pipeline.sh` | Pipeline orchestration (dry-run → triage → verify → real run) |
| `tools/auto-rebase/rebase-decision.sh` | Target selection logic |
| `tools/auto-rebase/resolve-config.sh` | Resolves target to config variables (also sources SSOT) |
| `tools/auto-rebase/conflict-triage.sh` | Conflict safety check |
| `tools/verify-rebase.sh` | Carry commit and downstream file verification |
| `tools/rebase-status/` | Go tool for querying repo state |
| `docs/version-matrix.md` | Generated upstream version mappings and wave composition |
| `.github/workflows/config-tests.yaml` | CI: verify-generate, syntax-check, config-load |

## Responding to CI Failures

When the Auto Rebase badge turns red, follow this process to diagnose and fix the failure.

### Step 1 — Identify the failed job

Click the badge or go to the [Actions tab](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml) and open the failed run. Each failing job is named `rebase (<branch>, <target>, <trigger>)`, e.g. `rebase (oadp-1.6, velero-plugin-for-aws-oadp-1.6, upstream-changed)`. Copy the job URL from the browser.

### Step 2 — Analyze the failure

Use the Claude Code skill to classify the failure and get a concrete fix proposal:

```
/analyze-rebase-failure <job-url>
```

The skill fetches the CI logs and classifies the failure into one of these categories:

| Category | Signature | Fix |
|----------|-----------|-----|
| **Cherry-pick conflict** | `WARNING - Upstream content may have been dropped from '<FILE>'` + `ERROR - Manual intervention is needed` | See [Manual rebase](#manual-rebase) below |
| **Hook script failure** | `Unable to run 'go vet'` / `Unable to run 'go mod tidy'` | Fix the failing code or hook; see [Manual rebase](#manual-rebase) if source conflicts caused broken code |
| **OWNERS conflict** | `'OWNERS' is not covered by any configured hook` | Add `OWNERS` to `expected_conflicts` for the repo in `repos.yaml` |
| **go.mod/go.sum only** | Triage says `safe=true` but retry still fails | Check wave ordering — the dependency may not be merged yet |
| **Infrastructure** | Image pull errors, auth failures, network timeouts | Re-run the failed jobs from the Actions tab |
| **Config or setup** | `Config file not found` / `Missing VAR from versions env` | Fix the config or versions file |

### Step 3 — Apply the fix

#### OWNERS conflict

Add the file to the repo's `expected_conflicts` list in `repos.yaml`:

```yaml
- org: migtools
  repo: oadp-vmdp
  expected_conflicts:
    - cli/app.go
    - OWNERS        # ← add this
```

Run `make test`, commit, and open a PR against `oadp-dev`.

#### Manual rebase

Use this when source code files conflict and the auto-resolution produces broken code (e.g. `go vet` fails after triage retries with `--conflict-policy warn`).

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

Common pattern: a carry commit was written against an older upstream. The newer upstream added fields or constants in the same area; the cherry-pick dropped them. Restore the upstream additions alongside the carry's own changes.

**4. Commit the fix:**

```bash
git add <fixed-files>
git commit -m "UPSTREAM: <carry>: Restore <description> dropped by rebase conflict resolution"
```

**5. Run missed hooks and update go modules:**

```bash
go mod tidy
git add go.mod go.sum
git commit -m "UPSTREAM: <drop>: Updating go modules"
```

If `normalize-dockerfiles-and-commit.sh` was the last hook and never ran, check whether any Dockerfiles need it:

```bash
grep -rE 'FROM\s+.*\bgolang:[0-9]+\.[0-9]+\.[0-9]+' --include='Dockerfile*' --include='Containerfile*' .
```

If there are matches, run the hook directly:

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
