# OADP Rebase

This repository manages rebases and dependency updates for ~20 downstream forks in the OADP (OpenShift API for Data Protection) ecosystem. It does not contain application code — it contains rebase configs, hook scripts, tooling, and CI workflows.

## Repository Structure

```
repos.yaml             # SSOT — repository definitions (org, wave, config prefix, images)
versions/              # SSOT — upstream tags and SHAs per OADP version
rebase-configs/        # Per-repo-per-branch rebase configuration files
rebasebot-hook-scripts/  # Post-rebase hooks (go-replace, go-mod-tidy, submodules, etc.)
run-oadp-rebase.sh     # Main entry point — config loading, wave definitions, rebasebot invocation
tools/
  auto-rebase/         # CI pipeline scripts (decision, triage, verify)
  rebase-status/       # Go tool for querying repo rebase state
  generate-go-replace-velero.sh  # Generates go-replace_velero hooks from SSOT
  generate-verify-tag-sha.sh  # Generates verify-tag-sha hooks from SSOT
  generate-version-matrix.sh  # Generates docs/version-matrix.md from SSOT
docs/                  # auto-rebase.md, version-matrix.md (generated)
Makefile               # generate, verify-generate, test targets
```

## Key Concepts

**Repos SSOT**: `repos.yaml` is the single source of truth for repository definitions — org, repo name, wave, config filename prefix, images, and branch constraints. All tooling (Go rebase-status, shell scripts, generators) reads this file directly via `yq`. To add a new repo, add it to `repos.yaml` and create its config file in `rebase-configs/`.

**Versions SSOT**: `versions/oadp-1.X.env` files are the single source of truth for upstream tags. Configs reference these variables. Hook scripts and docs are generated from them. Never hardcode a tag or SHA — update the versions file and run `make generate`.

**Wave ordering**: Repos rebase in waves (1–5). Each wave depends on the previous one being merged. Wave composition is defined in `repos.yaml` and varies per OADP version via `min_branch`/`max_branch` fields — see `docs/version-matrix.md`.

**Two rebase patterns**: Full upstream rebase (source != dest) cherry-picks downstream commits onto a new upstream tag. Hooks-only rebase (source == dest) just runs hooks to update dependencies like go.mod replaces.

**Hook scripts**: Run by rebasebot after each rebase. Most are version-specific (e.g., `go-replace_velero_oadp-1.6.sh`) because they contain the downstream branch name. The `go-replace_velero_*.sh` and `verify-tag-sha_*.sh` hooks are generated from the SSOT.

## Common Tasks

```bash
make test              # Run all validation checks
make generate          # Regenerate hooks + docs from versions SSOT
./run-oadp-rebase.sh -t velero-oadp-1.6           # Test config loading
./run-oadp-rebase.sh --dry-run --local-hooks \
  --working-dir ~/rebase-workdir \
  -s ~/.rebasebot/secrets velero-oadp-1.6          # Dry-run with rebasebot
```

## Rules

- Always run `make test` before committing changes to configs, hooks, or versions files
- Never edit `go-replace_velero_*.sh`, `verify-tag-sha_*.sh`, `docs/version-matrix.md`, or `tools/cve-scan/image-map.sh` by hand — they are generated
- When changing a version, update only `versions/oadp-1.X.env` then `make generate`
- When adding/changing repos, update only `repos.yaml` — all tooling reads it directly
- Config files use `${VAR:?error}` guards — if a versions file is missing, they fail fast
- Shell scripts require `yq` (Mike Farah's) for reading repos.yaml
