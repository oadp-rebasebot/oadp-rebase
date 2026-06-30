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
KUBEVIRT_PLUGIN_TAG="v0.9.0"
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
