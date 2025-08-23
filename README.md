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

## Summary

This structured **🌊 wave** approach allows for:

1. **Fewer conflicts** Independent components are updated first, so there’s less chance of breaking anything.
2. **Scheduled updates** Each wave can be run at any time. Rebasebot will only create a Pull Request if the dependent repository has been updated or rebased, ensuring that changes are propagated correctly across all waves.
3. **CI at any time** All Pull Requests trigger Prow and GitHub Actions, so developers and QA can run automated tests (mandatory and optional) even before the rebase is applied.
4. **Predictable and traceable updates** Each wave clearly shows which components depend on others and when changes are applied. This makes it easy to understand the update sequence and allows developers to step in if any merge conflicts can’t be automatically resolved.
5. **Frequent rebases and updates** Running rebases more often helps catch merge conflicts and potential compatibility issues early, reducing the risk of problems accumulating over time.

> By following this plan, oadp-team can safely perform rebases and updates across multiple repositories in a predictable, auditable manner.
