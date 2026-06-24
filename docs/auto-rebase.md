# Auto-Rebase System

The auto-rebase system runs daily via GitHub Actions to keep downstream OADP forks in sync with their upstreams. It inspects the current state of every tracked repository, decides which ones need a rebase, runs rebasebot against them, and notifies the team of the results.

This document explains the architecture, the decision logic, and the concrete steps for adding or removing a release branch.

## How It Works

The system is driven by two GitHub Actions workflows that share a set of shell scripts, Go tooling, and configuration files.

### Auto-Rebase Workflow (`.github/workflows/auto-rebase.yaml`)

Runs on a daily cron (07:00 UTC) or on manual dispatch. The workflow has three jobs:

1. **decide** — Builds the Go `rebase-status` tool, runs it against each configured branch to produce a JSON status report, then pipes that into `rebase-decision.sh` to select which repos need a rebase. The output is a GitHub Actions matrix of `{branch, target, reason}` objects.

2. **rebase** — Fans out across the matrix. For each target, it runs `run-oadp-rebase.sh` with `--conflict-policy strict`. If the strict run fails, `conflict-triage.sh` inspects the rebasebot output to decide whether the conflicting files are all covered by post-rebase hooks. If so, it retries with `--conflict-policy warn`. After a successful rebase, it posts `/ok-to-test` on the created PR.

3. **notify** — Collects result artifacts and sends a Slack summary via `rebase-notify.sh`.

### Status & Wiki Workflow (`.github/workflows/rebase-status-wiki.yaml`)

Runs after every auto-rebase completion, on a weekday cron, or on manual dispatch. It generates markdown status reports for each branch, pushes them to the GitHub wiki, deploys a dependency DAG visualization to GitHub Pages, and optionally sends a Slack status digest.

### Decision Logic (`tools/auto-rebase/rebase-decision.sh`)

The decision script reads `rebase-status --json` from stdin and applies these rules per repo:

1. Skip if `skip == true` in the config
2. Skip if no rebase config exists (`checks.config.status != "ok"`)
3. Skip if a rebase PR is already open (`checks.open_pr.status == "ok"`)
4. **Wave 1 repos** are always eligible (they have no internal OADP dependencies)
5. **Wave 2+ repos** are eligible only when their dependencies have moved ahead (`checks.dep_sync.status == "fail"`)

This means the system is self-sequencing: wave 2 repos won't rebase until wave 1 merges, wave 3 waits for wave 2, and so on.

### Conflict Triage (`tools/auto-rebase/conflict-triage.sh`)

When rebasebot fails with strict conflict policy, this script checks whether every file flagged in the WARNING output is covered by the repo's configured post-rebase hook scripts (go.mod, go.sum, Dockerfiles, lock files). If all conflicts are in hook-managed files, it returns `safe: true` and the workflow retries with `--conflict-policy warn`, letting the hooks fix those files on the next pass.

## Configuration Levers

The system's behavior is controlled by several files that must stay in sync. When adding or removing a release branch, each of these needs to be updated.

### 1. Repo Registry (`tools/rebase-status/registry.go`)

The `allRepos` slice is the canonical catalog of every repository in the OADP ecosystem. Each entry specifies the org, repo name, wave number, and optional flags:

- **`MinBranch`** — The earliest release branch this repo participates in. Repos without this field exist on all branches. For example, `filebrowser` has `MinBranch: "oadp-1.6"` and won't appear in status reports for oadp-1.5 or earlier.
- **`MainOnly`** — The repo only tracks `main`, never release branches (e.g. `udistribution`).
- **`NoRebase`** — Not managed by rebasebot; only tracked for dependency/build status.

The `filenameToRepo` map and `repoImages` map in the same file also need entries when onboarding a new repository (not needed for new branches of existing repos).

### 2. Rebase Config Files (`rebase-configs/`)

One shell env file per repo per branch, named `{prefix}_{branch}.env.sh`. These define the upstream source, downstream destination, intermediate rebase fork, and which hook scripts to run. For example:

```
rebase-configs/migtools_kopia_oadp-1.6.env.sh
rebase-configs/openshift_velero_oadp-1.6.env.sh
```

Each file sets variables like `SOURCE_UPSTREAM_REPO`, `DESTINATION_DOWNSTREAM_REPO`, `REBASE_REPO`, `HOOK_SCRIPTS`, and optionally `SKIP_REPO=true`.

When adding a new branch, you need a config file for every repo that participates in that branch. The easiest approach is to copy the configs from the closest existing branch and update the branch names and upstream tags.

### 3. Hook Scripts (`rebasebot-hook-scripts/`)

Post-rebase hook scripts that run after rebasebot completes its cherry-pick. Many are branch-specific because they contain hardcoded dependency versions or replace directives:

- `go-replace_velero_oadp-1.6.sh` — Adds `go.mod` replace directives pointing to the downstream velero fork for that branch.
- `go-use-tag_oadp-operator_oadp-1.6.sh` — Updates `go.mod` to use specific downstream tags.
- `kopia-submodule-and-commit_oadp-1.6.sh` — Updates git submodule references.

Generic hooks like `go-mod-tidy-and-commit.sh` and `normalize-dockerfiles-and-commit.sh` are branch-agnostic and don't need copies.

### 4. Rebase Runner Script (`run-oadp-rebase.sh`)

This script has two branch-aware sections:

- **`get_config_name()`** — A case statement mapping human-friendly target names (e.g. `velero-oadp-1.6`) to config file prefixes. Every repo-branch combination needs an entry here.
- **`get_wave_repos()`** — Per-branch wave definitions listing which targets belong to each wave. Each branch has its own elif block.

### 5. Auto-Rebase Workflow Branch List (`.github/workflows/auto-rebase.yaml`)

Line 58-59 in the `decide` job defines which branches the daily cron processes:

```yaml
if [ "$INPUT_BRANCHES" = "all" ] || [ -z "$INPUT_BRANCHES" ]; then
  BRANCHES="oadp-1.6"
```

This is the default when triggered by schedule or by manual dispatch with `branches: "all"`. Add new branches here (space-separated) to include them in the daily run.

### 6. Status Wiki Workflow Branch List (`.github/workflows/rebase-status-wiki.yaml`)

Two env vars at the top of the file:

```yaml
env:
  BRANCHES: oadp-1.6 oadp-1.5 oadp-1.4 oadp-1.3
  NOTIFY_BRANCHES: oadp-1.6
```

- **`BRANCHES`** controls which branches get wiki pages and DAG visualizations.
- **`NOTIFY_BRANCHES`** controls which branches are included in the Slack status digest.

### 7. Slack Notification Sort Order (`tools/auto-rebase/rebase-notify.sh`)

The `rebase-notify.sh` script has a hardcoded branch sort order (around line 92) used to group results in Slack messages:

```jq
elif . == "oadp-1.6" then 1
elif . == "oadp-1.5" then 2
```

Add new branches here so they sort correctly in notifications.

## Adding a New Release Branch

When OADP cuts a new release (e.g. `oadp-1.7`), follow these steps:

1. **Create config files** — For each repo that participates in the new branch, create a `rebase-configs/{prefix}_oadp-1.7.env.sh` file. Copy from the previous release branch and update:
   - `DESTINATION_DOWNSTREAM_BRANCH` and the `:branch` suffix in `DESTINATION_DOWNSTREAM_REPO`
   - `SOURCE_UPSTREAM_REPO` to point at the correct upstream tag/branch for this release
   - The `:rebase-bot-` branch suffix in `REBASE_REPO`
   - Check `MinBranch` in `registry.go` to know which repos to skip

2. **Create branch-specific hook scripts** — Copy and update any `go-replace_*`, `go-use-tag_*`, and submodule hook scripts that reference specific branches or tags. Update the dependency versions inside them to match the new release.

3. **Update `run-oadp-rebase.sh`** — Add entries to `get_config_name()` for each new repo-branch target, and add a new `elif` block in `get_wave_repos()` listing the wave composition for the new branch.

4. **Update `auto-rebase.yaml`** — Add `oadp-1.7` to the `BRANCHES` default in the `decide` job if you want the daily cron to process it.

5. **Update `rebase-status-wiki.yaml`** — Add `oadp-1.7` to the `BRANCHES` env var (and optionally to `NOTIFY_BRANCHES`).

6. **Update `rebase-notify.sh`** — Add a sort entry for `oadp-1.7` in the branch ordering.

No changes to `registry.go` are needed unless you're also onboarding a new repository that didn't exist before.

## Removing a Release Branch

When a branch reaches end-of-life:

1. **Remove from workflow branch lists** — Delete the branch from `BRANCHES` in both `auto-rebase.yaml` and `rebase-status-wiki.yaml`.

2. **Remove from `rebase-notify.sh`** — Delete the sort order entry.

3. **Clean up `run-oadp-rebase.sh`** — Remove the branch's entries from `get_config_name()` and delete its `elif` block from `get_wave_repos()`.

4. **Optionally remove config files and hook scripts** — The config files in `rebase-configs/` and branch-specific hook scripts can be deleted or left in place. They won't be used if the branch isn't in any workflow's branch list, but removing them keeps the directory clean.
