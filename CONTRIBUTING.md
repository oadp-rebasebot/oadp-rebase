# Contributing to OADP Rebase

## Prerequisites

Install these tools before working on this repository:

- **[yq](https://github.com/mikefarah/yq)** (Mike Farah's, not Python yq) — used by all shell tooling to read `repos.yaml`
- **[Go](https://go.dev/dl/)** 1.25+ — for building and testing the tools in `tools/`
- **[ShellCheck](https://www.shellcheck.net/)** — for linting shell scripts (`make shellcheck`)
- **[gh](https://cli.github.com/)** — GitHub CLI, used by CI scripts and the rebase-status tool
- **[jq](https://jqlang.github.io/jq/)** — used by several shell scripts for JSON processing

For running actual rebases (not required for config/hook changes):

- **[podman](https://podman.io/)** or **docker** — rebasebot runs in a container
- **rebasebot** — the rebase engine itself; see the [rebasebot repo](https://github.com/openshift-eng/rebasebot)
- A `~/.rebasebot/secrets/` directory with GitHub tokens

## Validation

Always run `make test` before committing. It checks:

- `repos.yaml` is valid and config prefixes have matching files
- Generated files match the SSOT (`make generate` + `git diff`)
- All shell scripts parse without errors (`bash -n`)
- Every config loads successfully for its branch
- The `resolve-config.sh` script maps targets to the correct config names
- Every hook script referenced in a config exists on disk
- Go unit tests pass for all tools
- The 90-test auto-rebase decision/triage test suite passes

Additionally, `make shellcheck` lints all scripts (run separately or in CI).

## Common Workflows

### Update an upstream tag

When a new upstream release comes out (e.g., Velero v1.18.3):

1. Edit `versions/oadp-1.X.env` — update the relevant `*_TAG` and `*_SHA` variables
2. Run `make generate` — regenerates hook scripts and `docs/version-matrix.md`
3. Run `make test` — validates everything is consistent
4. Commit both the versions file and all generated files

### Add a new repository

1. Add the repo entry to `repos.yaml` with org, repo name, wave, config_prefix, and optionally `min_branch`, `max_branch`, `dev_branch`, `main_only`, or `images`
2. Create config files in `rebase-configs/` for each branch the repo supports, named `<config_prefix>_<branch>.env.sh`
3. Run `make generate` — updates `docs/version-matrix.md` and may generate new hooks
4. Run `make test`

### Add a new OADP version (e.g., oadp-1.7)

1. Create `versions/oadp-1.7.env` with all upstream tag variables
2. Create config files in `rebase-configs/` for each repo that has the new branch
3. Run `make generate` — creates version-specific hooks for the new branch and updates the version matrix
4. Run `make test`

### Add a new hook script

If the hook follows an existing pattern (go-replace, submodule, go-use-tag, copy-crds), add it to the appropriate generator in `tools/generate-*.sh` rather than creating a hand-maintained file.

For a genuinely new hook type:

1. Create the script in `rebasebot-hook-scripts/`
2. Reference it from the relevant config files in `rebase-configs/`
3. Run `make test` — `verify-hooks` confirms the referenced file exists

### Modify a generated hook

Never edit generated files directly. Instead:

1. Modify the generator script in `tools/generate-*.sh`
2. Run `make generate`
3. Review the changes across all generated files
4. Run `make test`

## Project Structure

See [CLAUDE.md](CLAUDE.md) for a concise overview. For deeper architecture details including data flow, the wave system, and hook execution, see [AGENTS.md](AGENTS.md).
