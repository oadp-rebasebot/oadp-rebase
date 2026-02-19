# OADP Rebase

This repository manages the rebases and updates of Velero and OADP-related components, ensuring that dependencies remain in sync and compatible.  
It includes scripts (hooks) used by the rebasebot during the rebase process, as well as mappings between upstream and downstream tags.

# Velero & OADP Dependency Rebase Process and Graph

This document outlines a structured plan for rebasing and updating Velero and its OADP dependencies. The process is organized into **🌊 waves**, providing a clear, predictable, and safe sequence for applying updates across multiple repositories.

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

- `openshift/docker-distribution`  
 └─🟠─ `migtools/udistribution`
- 🔵 `migtools/kopia`
- 🔵 `openshift/restic`
- 🔵 `migtools/kubevirt-velero-plugin`
- 🔵 `migtools/filebrowser`


> **Note:** `migtools/udistribution` requires only a tag update.  
> **Note:** `migtools/kopia` repository must be rebased from the same upstream branch or tag referenced in Velero's `go.mod`. This alignment is automatically handled by the relevant scripts in the [`rebase-configs`](./rebase-configs) directory.

---

## 🌊 II Wave

The second wave rebases Velero and integrates with the kopia and restic dependencies prepared in the first wave:

- `migtools/kopia`  
  `openshift/restic`  
 └─🔵─ `openshift/velero` (restic as submodule in `.gitmodules`)

> **Note:** This wave introduces a full rebase for Velero, including `go.mod` updates.

---

## 🌊 III Wave

The third wave rebases and updates Velero plugins and updates the OADP operator:

- `openshift/velero`  
  `migtools/kopia`  
 └─🔵─ `openshift/velero-plugin-for-csi`

- `openshift/velero`  
 ├─🟠─ `openshift/oadp-operator`  
 ├─🔵─ `openshift/velero-plugin-for-aws`  
 ├─🔵─ `openshift/velero-plugin-for-legacy-aws`  
 └─🔵─ `openshift/velero-plugin-for-microsoft-azure`  

> **Note:** `velero-plugin-for-csi` requires both Velero and Kopia as dependencies.

---

## 🌊 IV Wave

The fourth wave focuses on Non-Admin OADP components:

- `openshift/velero`  
 └─🟠─ `migtools/oadp-non-admin`# `go.mod` replace + update

- `openshift/oadp-operator`  
 └─🟢─ `migtools/oadp-non-admin`# only tag update

- `openshift/oadp-operator`  
  `migtools/udistribution`  
 └─🟢─ `openshift/openshift-velero-plugin`# only tag update

- `openshift/velero`  
  `openshift/docker-distribution/v3`  
 └─🟠─ `openshift/openshift-velero-plugin`# `go.mod` replace + update

> **Note:** This wave is blocked only by the `openshift/oadp-operator` update from Wave III.

---

## 🌊 V Wave

The final wave updates the OADP Must-Gather components:

- `openshift/velero`  
 └─🟠─ `openshift/oadp-must-gather`

- `openshift/oadp-operator`  
  `migtools/oadp-non-admin`  
 └─🟢─ `openshift/oadp-must-gather`

> **Note:** This wave is effectively gated only by the `migtools/oadp-non-admin` update from previous IV Wave; all other components are already ready.

---

## Using the Rebase Script

The `run-oadp-rebase.sh` script provides a unified interface for running rebase operations.

### Prerequisites

1. **Secrets Directory**: Create `~/.rebasebot/secrets/` with GitHub App private keys:
   - `oadp-rebasebot-app-key`
   - `oadp-rebasebot-cloner-key`

2. **Container Runtime**: Install Docker or Podman

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
