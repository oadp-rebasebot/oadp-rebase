# OADP Rebase

[![Auto Rebase](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml/badge.svg?branch=oadp-dev)](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml)
[![Prow Audit Status](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/prow-audit-status.yaml/badge.svg?branch=oadp-dev)](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/prow-audit-status.yaml)
[![Deploy Prow Audit Page](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/prow-audit-pages.yaml/badge.svg?branch=oadp-dev)](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/prow-audit-pages.yaml)
[![Rebase Status Wiki](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/rebase-status-wiki.yaml/badge.svg?branch=oadp-dev)](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/rebase-status-wiki.yaml)

This repository manages the rebases and updates of Velero and OADP-related components, ensuring that dependencies remain in sync and compatible.  
It includes scripts (hooks) used by the rebasebot during the rebase process, as well as mappings between upstream and downstream tags.

See [CONTRIBUTING.md](CONTRIBUTING.md) for prerequisites and development workflows. For the authoritative branch coverage and wave composition per OADP version, see the generated [version matrix](docs/version-matrix.md).

# OADP Rebase Pages

Quick navigation to the pages published to GitHub Pages for the
[`oadp-rebasebot/oadp-rebase`](https://github.com/oadp-rebasebot/oadp-rebase)
repository. The site is assembled by
[`tools/assemble-pages.sh`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/tools/assemble-pages.sh)
and deployed to <https://oadp-rebasebot.github.io/oadp-rebase/>.

## Site pages

| Page | Link | Assembled from |
| --- | --- | --- |
| OADP Prow Merge Bot Configuration Audit | <https://oadp-rebasebot.github.io/oadp-rebase/> | [`tools/prow-merge-bot-configs/index.html`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/tools/prow-merge-bot-configs/index.html) |
| OADP Rebase Dependency Graph (DAG) | <https://oadp-rebasebot.github.io/oadp-rebase/rebase-dag/> | [`tools/rebase-dag/`](https://github.com/oadp-rebasebot/oadp-rebase/tree/oadp-dev/tools/rebase-dag) |
| OADP Rebase Dashboard | <https://oadp-rebasebot.github.io/oadp-rebase/rebase-dashboard/> | [`tools/rebase-dashboard/`](https://github.com/oadp-rebasebot/oadp-rebase/tree/oadp-dev/tools/rebase-dashboard) |

## Related generators

The following scripts and workflows produce data or content consumed by the pages above:

- [`tools/assemble-pages.sh`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/tools/assemble-pages.sh) — assembles the full `_site` directory from the sources listed above.
- [`.github/workflows/prow-audit-pages.yaml`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/.github/workflows/prow-audit-pages.yaml) — builds the dashboard and deploys the assembled site to GitHub Pages.
- [`.github/workflows/prow-audit-status.yaml`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/.github/workflows/prow-audit-status.yaml) — generates the Prow audit status badge (`badge.json`).
- [`.github/workflows/rebase-status-wiki.yaml`](https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/.github/workflows/rebase-status-wiki.yaml) — generates the DAG data (`dag-data/*.json`) shared by the DAG and Dashboard pages.


# Velero & OADP Dependency Rebase Process and Graph

This document outlines a structured plan for rebasing and updating Velero and its OADP dependencies. The process is organized into **🌊 waves**, providing a clear, predictable, and safe sequence for applying updates across multiple repositories.

> **Note:** The wave graph below shows the superset of all repos across all OADP versions. For the exact per-version wave composition (which repos are in which wave for each branch), see the generated [version matrix](docs/version-matrix.md).

---

## Rebase Graph Legend

The graph uses **colored icons** to indicate the type of update:

- 🔵 — Full rebase from upstream + `go.mod` replace + `go.mod` update tags
- 🟠 — `go.mod` replace + `go.mod` update tags
- 🟢 — `go.mod` update tags only

These icons help to quickly identify the impact of each update, from full rebase to simple tag updates.

---

## 🌊 I Wave

The first wave focuses on independent core dependencies without requiring `go.mod` replacements:

- [`openshift/docker-distribution`](https://github.com/openshift/docker-distribution)  
 └─🟠─ [`migtools/udistribution`](https://github.com/migtools/udistribution)
- 🔵 [`migtools/kopia`](https://github.com/migtools/kopia)  
 └─🔵─ [`migtools/oadp-vmdp`](https://github.com/migtools/oadp-vmdp)
- 🔵 [`openshift/restic`](https://github.com/openshift/restic)
- 🔵 [`migtools/filebrowser`](https://github.com/migtools/filebrowser)


> **Note:** [`migtools/udistribution`](https://github.com/migtools/udistribution) requires only a tag update.
> **Note:** [`migtools/kopia`](https://github.com/migtools/kopia) repository must be rebased from the same upstream branch or tag referenced in Velero's `go.mod`. This alignment is automatically handled by the relevant scripts in the [`rebase-configs`](./rebase-configs) directory.
> **Note:** [`migtools/oadp-vmdp`](https://github.com/migtools/oadp-vmdp) is rebased directly from the downstream `migtools/kopia` fork.

---

## 🌊 II Wave

The second wave rebases Velero and integrates with the kopia and restic dependencies prepared in the first wave:

- [`migtools/kopia`](https://github.com/migtools/kopia)  
  [`openshift/restic`](https://github.com/openshift/restic)  
 └─🔵─ [`openshift/velero`](https://github.com/openshift/velero) (restic as submodule in `.gitmodules`)

> **Note:** This wave introduces a full rebase for Velero, including `go.mod` updates.

---

## 🌊 III Wave

The third wave rebases and updates Velero plugins and updates the OADP operator:

- [`openshift/velero`](https://github.com/openshift/velero)  
 ├─🔵─ [`migtools/kubevirt-velero-plugin`](https://github.com/migtools/kubevirt-velero-plugin)  
 ├─🔵─ [`openshift/velero-plugin-for-csi`](https://github.com/openshift/velero-plugin-for-csi) *(also requires [`migtools/kopia`](https://github.com/migtools/kopia))*  
 ├─🟠─ [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator)  
 ├─🔵─ [`openshift/velero-plugin-for-aws`](https://github.com/openshift/velero-plugin-for-aws)  
 ├─🔵─ [`openshift/velero-plugin-for-gcp`](https://github.com/openshift/velero-plugin-for-gcp)  
 ├─🔵─ [`openshift/velero-plugin-for-legacy-aws`](https://github.com/openshift/velero-plugin-for-legacy-aws)  
 ├─🔵─ [`openshift/velero-plugin-for-microsoft-azure`](https://github.com/openshift/velero-plugin-for-microsoft-azure)  
 └─🟠─ [`openshift/hypershift-oadp-plugin`](https://github.com/openshift/hypershift-oadp-plugin)  

> **Note:** [`velero-plugin-for-csi`](https://github.com/openshift/velero-plugin-for-csi) requires both Velero and Kopia as dependencies.

---

## 🌊 IV Wave

The fourth wave focuses on Non-Admin OADP components:

- [`openshift/velero`](https://github.com/openshift/velero)  
 ├─🟠─ [`migtools/oadp-non-admin`](https://github.com/migtools/oadp-non-admin)# `go.mod` replace + update  
 ├─🟠─ [`openshift/openshift-velero-plugin`](https://github.com/openshift/openshift-velero-plugin) *(also requires [`openshift/docker-distribution/v3`](https://github.com/openshift/docker-distribution))*# `go.mod` replace + update  
 ├─🟠─ [`migtools/kubevirt-datamover-controller`](https://github.com/migtools/kubevirt-datamover-controller)  
 └─🟠─ [`migtools/oadp-vm-file-restore`](https://github.com/migtools/oadp-vm-file-restore)  

- [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator)  
 ├─🟢─ [`migtools/oadp-non-admin`](https://github.com/migtools/oadp-non-admin)# only tag update  
 └─🟢─ [`openshift/openshift-velero-plugin`](https://github.com/openshift/openshift-velero-plugin) *(also requires [`migtools/udistribution`](https://github.com/migtools/udistribution))*# only tag update  

> **Note:** This wave is blocked only by the [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator) update from Wave III.

---

## 🌊 V Wave

The final wave updates the OADP Must-Gather and CLI components:

- [`openshift/velero`](https://github.com/openshift/velero)  
 ├─🟠─ [`openshift/oadp-must-gather`](https://github.com/openshift/oadp-must-gather)  
 ├─🟠─ [`migtools/oadp-cli`](https://github.com/migtools/oadp-cli) *(also requires [`migtools/kopia`](https://github.com/migtools/kopia), [`migtools/oadp-non-admin`](https://github.com/migtools/oadp-non-admin), [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator))*  
 └─🟠─ [`migtools/kubevirt-datamover-plugin`](https://github.com/migtools/kubevirt-datamover-plugin) *(also requires [`migtools/kubevirt-datamover-controller`](https://github.com/migtools/kubevirt-datamover-controller))*  

- [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator)  
  [`migtools/oadp-non-admin`](https://github.com/migtools/oadp-non-admin)  
 └─🟢─ [`openshift/oadp-must-gather`](https://github.com/openshift/oadp-must-gather)  

> **Note:** This wave is effectively gated only by the [`migtools/oadp-non-admin`](https://github.com/migtools/oadp-non-admin) update from previous IV Wave; all other components are already ready.

---

## Using the Rebase Script

The `run-oadp-rebase.sh` script provides a unified interface for running rebase operations.

### Prerequisites

1. **Secrets Directory**: Create `~/.rebasebot/secrets/` with GitHub App private keys:
   - `oadp-rebasebot-app-key`
   - `oadp-rebasebot-cloner-key`

2. **Container Runtime**: Install Docker or Podman

### macOS Troubleshooting

1. **Podman VM clock drift**: After macOS sleep/hibernate, the podman VM clock can drift significantly, causing GitHub App JWT authentication to fail with `401 Bad credentials`. Fix with:
   ```bash
   podman machine ssh "sudo date -s @$(date +%s)"
   ```
Open issue: https://github.com/containers/podman/issues/27293

2. **`/tmp` volume mounts fail**: macOS's `/tmp` is a symlink to `/private/tmp`, which podman can't resolve for volume mounts. Use `--working-dir` with a path under `$HOME` instead (e.g. `--working-dir ~/.cache/oadp-rebase-workdir`), or use `/private/tmp` directly.

### Basic Usage

```bash
# Run single repository rebase
./run-oadp-rebase.sh -b oadp-dev kopia

# Run entire wave
./run-oadp-rebase.sh -w 1

# Dry run (preview changes without applying)
./run-oadp-rebase.sh -d -w 2

# Test configuration locally
./run-oadp-rebase.sh -t -b oadp-dev kopia
```

### Common Options

- `-d, --dry-run` - Preview changes without applying them
- `-w, --wave` - Run an entire wave (1-5)
- `-b, --branch BRANCH` - Specify target branch (default: oadp-dev)
- `-t, --test` - Test configuration only (no rebase)
- `-r, --remote` - Use remote configuration files
- `-s, --secrets-dir DIR` - Custom secrets directory

### Examples

```bash
# Rebase kopia for oadp-dev branch
./run-oadp-rebase.sh -b oadp-dev kopia

# Rebase kopia for oadp-1.5 branch
./run-oadp-rebase.sh -b oadp-1.5 kopia

# Run wave 1 with dry-run
./run-oadp-rebase.sh -d -w 1

# Run wave 2 for oadp-1.5 branch
./run-oadp-rebase.sh -b oadp-1.5 -w 2

# Test velero configuration
./run-oadp-rebase.sh -t -b oadp-dev velero
```

---

## Manual Intervention Guide

When rebasebot runs with `--conflict-policy strict` and detects that upstream content was lost during cherry-picking, it stops with:

```
ERROR - Manual intervention is needed to rebase <source> into <dest>
```

The working directory (`.rebase/`) is preserved with three remotes configured:
- `source` — upstream repository
- `dest` — downstream repository
- `rebase` — intermediate fork for PRs

### Understanding the Problem

Rebasebot starts from `source/<upstream-branch>` (current upstream) and cherry-picks all downstream-only commits on top. The downstream history often includes **old merge commits** from previous rebasebot runs (e.g., "Merge upstream:main into oadp-dev"). When these are cherry-picked with `-Xtheirs`, they can downgrade dependencies back to old versions because the merge commit's conflict resolution predates the current upstream.

### Fix Steps

```bash
# 1. Enter the working directory
cd .rebase

# 2. Check the git log to find the last good commit (before the problematic merge cherry-pick)
git log --oneline -10

# 3. Reset to the commit before the bad cherry-pick
#    (the last meaningful downstream commit, before old merge commits)
git reset --hard <last-good-commit>

# 4. Verify upstream content was restored
git diff source/<upstream-branch> -- Dockerfile
git diff source/<upstream-branch> -- go.mod
# These should be empty or show only intentional downstream differences

# 5. Run the post-rebase hooks manually using local checkout
#    Check the repo's rebase-config .env.sh file to identify which hooks are needed.
#    Hook scripts are located in rebasebot-hook-scripts/ in this repository.
#    Example for a repo that uses go-replace and go-mod-tidy hooks:

# Run the go-replace hook (adds go.mod replace directives for downstream dependencies)
../rebasebot-hook-scripts/go-replace_velero_<branch>.sh

# Run the go-mod-tidy hook (runs go mod tidy, go vet, and commits)
REBASEBOT_SOURCE=<upstream-branch> \
REBASEBOT_GIT_USERNAME="oadp-team-rebase-bot" \
REBASEBOT_GIT_EMAIL="oadp-maintainers@redhat.com" \
../rebasebot-hook-scripts/go-mod-tidy-and-commit.sh

# 6. Verify the result
git log --oneline -5
git status

# 7. Set up auth for pushing (if needed)
gh auth setup-git
# Or use SSH:
# git remote set-url rebase git@github.com:oadp-rebasebot/<repo>.git

# 8. Push to the rebase branch
git push rebase HEAD:rebase-bot-<branch> --force
```

> **Note:** Check the corresponding config in `rebase-configs/` (e.g., `openshift_velero_plugin_for_gcp_oadp-dev.env.sh`) to see which hook scripts are required for each repository and branch.

> **Note:** After pushing, you can create a PR manually from the rebase fork to the downstream repo, or re-run rebasebot — it will detect the existing branch and create/update the PR automatically.

> **Tip:** To prevent rebasebot from overwriting your manual fixes, add the `rebase/manual` label to the PR.

---

## Summary

This structured **🌊 wave** approach allows for:

1. **Fewer conflicts** Independent components are updated first, so there’s less chance of breaking anything.
2. **Scheduled updates** Each wave can be run at any time. Rebasebot will only create a Pull Request if the dependent repository has been updated or rebased, ensuring that changes are propagated correctly across all waves.
3. **CI at any time** All Pull Requests trigger Prow and GitHub Actions, so developers and QA can run automated tests (mandatory and optional) even before the rebase is applied.
4. **Predictable and traceable updates** Each wave clearly shows which components depend on others and when changes are applied. This makes it easy to understand the update sequence and allows developers to step in if any merge conflicts can’t be automatically resolved.
5. **Frequent rebases and updates** Running rebases more often helps catch merge conflicts and potential compatibility issues early, reducing the risk of problems accumulating over time.

> By following this plan, oadp-team can safely perform rebases and updates across multiple repositories in a predictable, auditable manner.
