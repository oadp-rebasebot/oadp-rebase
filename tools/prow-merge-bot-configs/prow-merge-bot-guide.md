# Prow Merge Bot Guide for OADP Repositories

[![Prow Audit Status](https://img.shields.io/endpoint?url=https://oadp-rebasebot.github.io/oadp-rebase/badge.json)](https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/prow-audit-status.yaml)

How Prow's merge automation (Tide), labeling bots (approve/lgtm plugins), and branch protection work together across OADP ecosystem repositories.

## Table of Contents

- [How Merging Works (Tide)](#how-merging-works-tide)
- [The Approve Plugin](#the-approve-plugin)
- [The LGTM Plugin](#the-lgtm-plugin)
- [Approve vs LGTM: Key Differences](#approve-vs-lgtm-key-differences)
- [OWNERS Files](#owners-files)
- [Branch Protection](#branch-protection)
- [Merge Requirements Summary](#merge-requirements-summary)
- [OADP Repo Configuration Audit](#oadp-repo-configuration-audit)
- [Configuration Reference](#configuration-reference)

---

## How Merging Works (Tide)

[Tide](https://docs.prow.k8s.io/docs/components/core-components/tide/) is Prow's merge controller. It continuously watches for PRs that meet all merge criteria and merges them automatically.

Tide merges a PR when **all** of the following are true:

1. The PR has all **required labels** (typically `approved` + `lgtm`)
2. The PR has **none of the blocking labels** (e.g., `do-not-merge/hold`, `needs-rebase`, `do-not-merge/work-in-progress`, `jira/invalid-bug`, etc.)
3. The PR targets a **branch included** in the Tide query (configured in `_prowconfig.yaml`)
4. All required **CI status checks** pass
5. The PR is **not in a merge conflict** state

### Tide Configuration (per-repo `_prowconfig.yaml`)

Each repo's `_prowconfig.yaml` defines which branches Tide watches, what labels are required, and what labels block merging:

```yaml
tide:
  queries:
  - includedBranches:
    - oadp-1.6
    - oadp-dev
    labels:        # ALL of these must be present
    - approved
    - lgtm
    missingLabels: # NONE of these may be present
    - do-not-merge/hold
    - do-not-merge/work-in-progress
    - do-not-merge/invalid-owners-file
    - needs-rebase
    - jira/invalid-bug
    - backports/unvalidated-commits
    repos:
    - org/repo
```

Some repos also use a `keep-main-query-separate` label in their Tide `missingLabels` list, which prevents PRs from being batched in the same Tide merge pool as PRs targeting main/master branches.

### Merge Methods

Tide can be configured to use different merge strategies per repo:

| Method | Description |
|--------|-------------|
| `merge` (default) | Standard merge commit |
| `squash` | Squash all commits into one |
| `rebase` | Rebase onto target branch |

Configured via:
```yaml
tide:
  merge_method:
    org/repo: squash
```

---

## The Approve Plugin

**Reference:** [Prow Approve Plugin](https://docs.prow.k8s.io/docs/components/plugins/approve/approvers/) | [Go doc](https://pkg.go.dev/sigs.k8s.io/prow/pkg/plugins#Approve)

The approve plugin manages the `approved` label based on OWNERS files. It ensures that all changed files have been approved by someone listed as an approver in the relevant OWNERS file(s).

### How It Works

1. When a PR is opened, the bot analyzes which files are changed
2. It determines which OWNERS files govern those files
3. It suggests a minimal set of approvers who can cover all changed files
4. Approvers comment `/approve` to indicate their approval
5. Once all changed file paths are covered by approvals, the bot adds the `approved` label

### Commands

| Command | Who Can Use | Effect |
|---------|-------------|--------|
| `/approve` | Approvers listed in relevant OWNERS files | Approves the files the user is an approver for |
| `/approve cancel` | Anyone who previously approved | Removes their approval |
| `/approve no-issue` | Approvers | Approves even when issue is required but missing |

### Key Configuration Options

```go
type Approve struct {
    Repos              []string  // Repos this config applies to
    RequireSelfApproval *bool    // If false (default in OADP), PR authors with
                                 // approval rights auto-approve their own PRs
    IssueRequired      bool      // Require linked issue for approval
    LgtmActsAsApprove  bool      // /lgtm also counts as /approve
    IgnoreReviewState  *bool     // Ignore GitHub review state (approve/request changes)
    CommandHelpLink    string    // Link shown in bot comments
}
```

#### `require_self_approval: false` (OADP default)

When `false`, if the PR author is listed as an approver in OWNERS for the changed files, the bot **automatically considers those files approved** by the author. The author does NOT need to separately `/approve` their own PR.

This is set to `false` across all OADP repos.

#### GitHub Review State Integration

By default (`ignore_review_state` not set or `false`), the approve plugin treats:
- A GitHub **"Approve" review** as equivalent to `/approve`
- A GitHub **"Request Changes" review** as equivalent to `/approve cancel`

---

## The LGTM Plugin

**Reference:** [Go doc](https://pkg.go.dev/sigs.k8s.io/prow/pkg/plugins#Lgtm)

The LGTM (Looks Good To Me) plugin manages the `lgtm` label. It represents that a code review has been completed.

### How It Works

1. A reviewer examines the PR code
2. They comment `/lgtm` to signal the code looks good
3. The bot adds the `lgtm` label
4. **If a new commit is pushed to the PR, the `lgtm` label is automatically removed** (the review must be re-done for the new code)

### Commands

| Command | Who Can Use | Effect |
|---------|-------------|--------|
| `/lgtm` | Any repo collaborator (or reviewer/approver in OWNERS) | Adds the `lgtm` label |
| `/lgtm cancel` | The person who gave lgtm, or anyone with approval rights | Removes the `lgtm` label |

### Key Configuration Options

```go
type Lgtm struct {
    Repos             []string  // Repos this config applies to
    ReviewActsAsLgtm  *bool     // GitHub "Approve" review also adds lgtm label
    StoreTreeHash     *bool     // Track tree hash to detect squash commits
    CommandHelpLink   string    // Link shown in bot comments
}
```

#### `review_acts_as_lgtm: true`

When enabled, submitting a GitHub **"Approve" review** (the green checkmark review in the GitHub UI) also counts as `/lgtm`, in addition to the `/lgtm` comment command. This is only enabled on **some** OADP repos (see audit below).

#### LGTM Label Retraction on New Commits

When a new commit is pushed to a PR that already has the `lgtm` label, the bot **automatically removes** the label. This ensures that the reviewer re-examines any new changes. If `store_tree_hash` is enabled, the bot can also detect squash commits.

---

## Approve vs LGTM: Key Differences

| Aspect | Approve (`approved` label) | LGTM (`lgtm` label) |
|--------|---------------------------|---------------------|
| **Purpose** | Ownership/acceptance gate | Code review quality gate |
| **Who can grant** | Only approvers in OWNERS files for the changed paths | Any repo collaborator |
| **Scope** | Per-directory (based on OWNERS file hierarchy) | Whole PR |
| **Auto-removed on new push** | No | **Yes** |
| **Author can self-grant** | Auto-granted if author is an approver (when `require_self_approval: false`) | **No** - authors cannot `/lgtm` their own PR |
| **GitHub review integration** | "Approve" review = `/approve` (default) | "Approve" review = `/lgtm` (only if `review_acts_as_lgtm: true`) |

### Typical Merge Flow

```
PR Opened
  │
  ├─ Author is an approver? ──yes──► `approved` label auto-added
  │                                    (when require_self_approval: false)
  │──no──► Approver comments /approve ──► `approved` label added
  │
  ├─ Reviewer comments /lgtm ──► `lgtm` label added
  │   (or submits GitHub "Approve" review if review_acts_as_lgtm: true)
  │
  ├─ CI passes
  │
  └─ No blocking labels present
       │
       ▼
  Tide merges the PR
```

---

## OWNERS Files

OWNERS files define who can approve and review code in each directory. They are hierarchical — a parent directory's approvers can approve files in subdirectories.

```yaml
# OWNERS
approvers:
- alice
- bob

reviewers:
- alice
- bob
- charlie
- diana
```

- **approvers**: Can `/approve` changes in this directory and its subdirectories
- **reviewers**: Suggested for code review by the blunderbuss plugin; can `/lgtm`

The approve bot uses a **set cover algorithm** to suggest the minimum number of approvers needed to cover all changed files across different directories.

---

## Branch Protection

Configured in `_prowconfig.yaml` under `branch-protection`. This controls GitHub-level branch protection settings:

```yaml
branch-protection:
  orgs:
    openshift:
      repos:
        oadp-operator:
          enforce_admins: true
          protect: true
          required_pull_request_reviews:
            dismiss_stale_reviews: true
            required_approving_review_count: 2
```

Key settings:
- **`enforce_admins`**: Even org admins must follow branch protection rules. See [why this is needed](#why-enforce_admins-is-required) below.
- **`required_approving_review_count`**: Number of GitHub approving reviews required (separate from Prow's approve/lgtm — this is a GitHub-native requirement)
- **`dismiss_stale_reviews`**: Dismiss approving reviews when new commits are pushed
- **`allow_force_pushes`**: Allow force-pushes to the branch (used by rebasebot for downstream rebases)

### Why `enforce_admins` Is Required

**`enforce_admins: true` is a workaround for a Prow/Tide limitation** ([kubernetes-sigs/prow#134](https://github.com/kubernetes-sigs/prow/issues/134)).

Tide does not natively enforce GitHub's `required_approving_review_count` setting. Without `enforce_admins`, Tide can merge a PR that has the `approved` and `lgtm` Prow labels even if it hasn't received the required number of GitHub approving reviews. This is because Tide uses the GitHub API to merge, and by default the merge API bypasses branch protection for admin-level tokens (which Prow's bot account typically has).

Setting `enforce_admins: true` closes this gap by forcing **all** merges — including those made by admin/bot accounts like Tide — to satisfy the `required_approving_review_count` before the merge API call succeeds.

**Without `enforce_admins: true`:**
```
PR gets approved + lgtm labels → Tide merges immediately
(GitHub review count requirement is silently bypassed)
```

**With `enforce_admins: true`:**
```
PR gets approved + lgtm labels → Tide attempts merge →
GitHub API rejects if required_approving_review_count not met →
PR stays open until enough GitHub reviews are submitted
```

This is why OADP-owned repos that set `required_approving_review_count` must also set `enforce_admins: true` — without it, the review count is effectively unenforced. Repos missing `enforce_admins` while having a review count requirement (see audit below) have a configuration gap where Tide can merge PRs without sufficient human reviews.

---

## Merge Requirements Summary

For a PR to merge in an OADP repo, it typically needs:

1. **`approved` label** — from the approve plugin (via `/approve` or author self-approval)
2. **`lgtm` label** — from the lgtm plugin (via `/lgtm` or GitHub Approve review where `review_acts_as_lgtm: true`)
3. **GitHub approving reviews** — the number required by `required_approving_review_count` (1 or 2 depending on repo)
4. **CI checks passing** — all required Prow jobs must succeed
5. **No blocking labels** — none of: `do-not-merge/hold`, `do-not-merge/work-in-progress`, `needs-rebase`, `jira/invalid-bug`, `do-not-merge/invalid-owners-file`, `backports/unvalidated-commits`
6. **Branch must be in Tide query** — the target branch must be listed in `includedBranches`

### Blocking a Merge

| Command | Effect |
|---------|--------|
| `/hold` | Adds `do-not-merge/hold` label |
| `/hold cancel` | Removes `do-not-merge/hold` label |
| `/wip` (or PR title starts with "WIP") | Adds `do-not-merge/work-in-progress` label |
| `/lgtm cancel` | Removes `lgtm` label |
| `/approve cancel` | Removes `approved` label |

---

## OADP Repo Configuration Audit

### Plugin Configuration Comparison

| Repository | `require_self_approval` | `review_acts_as_lgtm` | Separate `lgtm:` section | `approve` in plugins |
|------------|------------------------|----------------------|--------------------------|---------------------|
| **openshift/velero** | `false` | not set (=false) | yes | yes |
| **openshift/oadp-operator** | `false` | not set (=false) | yes | yes |
| **openshift/oadp-must-gather** | `false` | **`true`** | yes | yes |
| **openshift/openshift-velero-plugin** | `false` | not set (=false) | yes | yes |
| **openshift/restic** | `false` | **`true`** | yes | yes |
| **openshift/velero-plugin-for-aws** | `false` | not set (=false) | yes | yes |
| **openshift/velero-plugin-for-csi** | `false` | not set (=false) | yes | yes |
| **openshift/velero-plugin-for-gcp** | `false` | not set (=false) | yes | yes |
| **openshift/velero-plugin-for-legacy-aws** | `false` | not set (=false) | yes | yes |
| **openshift/velero-plugin-for-microsoft-azure** | `false` | not set (=false) | yes | yes |
| **openshift/hypershift-oadp-plugin** | `false` | **`true`** | yes | yes |
| **migtools/filebrowser** | `false` | not set (=false) | yes | yes |
| **migtools/kubevirt-datamover-controller** | `false` | not set (=false) | yes | yes |
| **migtools/kubevirt-datamover-plugin** | `false` | not set (=false) | yes | yes |
| **migtools/kubevirt-velero-plugin** | `false` | **`true`** | yes | yes |
| **migtools/oadp-cli** | `false` | not set (=false) | yes | yes |
| **migtools/oadp-non-admin** | `false` | not set (=false) | yes | yes |
| **migtools/oadp-vm-file-restore** | `false` | not set (=false) | yes | yes |
| **migtools/udistribution** | `false` | not set (=false) | **no** | yes |
| **migtools/velero-plugin-for-vsm** | `false` | not set (=false) | yes | yes |
| **migtools/volume-snapshot-mover** | `false` | not set (=false) | yes | yes |
| **migtools/oadp-vmdp** | `false` | not set (=false) | yes | yes |
| **migtools/kopia** | `false` | not set (=false) | yes | yes |

### Branch Protection Comparison

| Repository | `enforce_admins` | `required_approving_review_count` | `allow_force_pushes` | `dismiss_stale_reviews` |
|------------|-----------------|----------------------------------|---------------------|------------------------|
| **openshift/velero** | not set | not set | `true` (include oadp-*) | not set |
| **openshift/oadp-operator** | `true` | `2` | not set | `true` |
| **openshift/oadp-must-gather** | `true` | `2` | not set | `true` |
| **openshift/openshift-velero-plugin** | `true` | `2` | not set | `true` |
| **openshift/restic** | not set | not set | `true` (per-branch) | not set |
| **openshift/velero-plugin-for-aws** | not set | not set | `true` (include oadp-*) | not set |
| **openshift/velero-plugin-for-csi** | not set | not set | `true` (per-branch) | not set |
| **openshift/velero-plugin-for-gcp** | not set | not set | `true` (include oadp-*) | not set |
| **openshift/velero-plugin-for-legacy-aws** | not set | not set | `true` (per-branch) | not set |
| **openshift/velero-plugin-for-microsoft-azure** | not set | not set | `true` (include oadp-*) | not set |
| **openshift/hypershift-oadp-plugin** | not set | not set | not set | not set |
| **migtools/filebrowser** | `true` | **`1`** | not set | `true` |
| **migtools/kubevirt-datamover-controller** | `true` | `2` | not set | `true` |
| **migtools/kubevirt-datamover-plugin** | `true` | `2` | not set | `true` |
| **migtools/kubevirt-velero-plugin** | `true` | `2` | not set | `true` |
| **migtools/oadp-cli** | `true` | `2` | not set | `true` |
| **migtools/oadp-non-admin** | `true` | `2` | not set | `true` |
| **migtools/oadp-vm-file-restore** | `true` | **`1`** | not set | `true` |
| **migtools/udistribution** | `true` | `2` | not set | `true` |
| **migtools/velero-plugin-for-vsm** | not set | `2` | `true` (per-branch) | `true` |
| **migtools/volume-snapshot-mover** | not set | `2` | `true` (per-branch) | `true` |
| **migtools/oadp-vmdp** | not set | not set | not set | not set |
| **migtools/kopia** | not set | not set | not set | not set |

### Merge Method Comparison

| Repository | Merge Method |
|------------|-------------|
| **openshift/openshift-velero-plugin** | `squash` |
| **migtools/udistribution** | `squash` |
| **migtools/velero-plugin-for-vsm** | `squash` |
| **migtools/volume-snapshot-mover** | `squash` |
| All others | `merge` (default) |

### Sync Issues Found

#### 1. `review_acts_as_lgtm` is inconsistent

Only 4 repos have `review_acts_as_lgtm: true`:
- `openshift/oadp-must-gather`
- `openshift/restic`
- `openshift/hypershift-oadp-plugin`
- `migtools/kubevirt-velero-plugin`

All other repos require the explicit `/lgtm` comment — a GitHub "Approve" review alone is **not enough** to add the `lgtm` label.

**Impact**: Contributors may submit a GitHub approving review expecting it to count as LGTM, but on most OADP repos it won't. They must also comment `/lgtm`.

#### 2. `migtools/udistribution` — missing `lgtm:` config section

Has `lgtm` in its plugin list but no separate `lgtm:` configuration section. The plugin is active but has no repo-specific configuration (e.g., `review_acts_as_lgtm` is not configurable without the section).

#### 3. Branch protection is split into two patterns

**Pattern A** (rebasebot-managed upstream forks): `allow_force_pushes: true`, no `enforce_admins`, no `required_approving_review_count`
- velero, velero-plugin-for-aws, velero-plugin-for-gcp, velero-plugin-for-microsoft-azure, restic, velero-plugin-for-csi, velero-plugin-for-legacy-aws

**Pattern B** (OADP-owned repos): `enforce_admins: true`, `required_approving_review_count: 2`, `dismiss_stale_reviews: true`
- oadp-operator, oadp-must-gather, openshift-velero-plugin, most migtools repos

This split is intentional — Pattern A repos need force pushes for rebasebot to update downstream branches from upstream, while Pattern B repos are directly maintained.

**Important:** `enforce_admins: true` is required alongside `required_approving_review_count` as a workaround for [kubernetes-sigs/prow#134](https://github.com/kubernetes-sigs/prow/issues/134) — without it, Tide bypasses GitHub's review count requirement. See [Why `enforce_admins` Is Required](#why-enforce_admins-is-required).

#### 4. Inconsistent `required_approving_review_count`

Most repos with branch protection require **2** approving reviews, but:
- `migtools/filebrowser`: requires only **1**
- `migtools/oadp-vm-file-restore`: requires only **1**

#### 5. Tide branch coverage gaps

`openshift/velero-plugin-for-csi` Tide only covers `oadp-1.0` through `oadp-1.3` + `oadp-dev` — it's missing `oadp-1.4`, `oadp-1.5`, `oadp-1.6`. However, this repo is `SKIP_REPO=true` in rebasebot, so it may be intentionally dormant. (CSI plugin was merged into Velero core starting in Velero 1.12 / OADP 1.4.)

---

## Configuration Reference

### File Locations in `openshift/release`

```
core-services/prow/02_config/<org>/<repo>/
├── _pluginconfig.yaml    # Plugin settings (approve, lgtm, external_plugins)
└── _prowconfig.yaml      # Tide queries, branch protection, merge methods
```

### Audit Tools

#### Go TUI (`tui/prow-audit-tui`)

Interactive terminal dashboard for auditing Prow configs.

```bash
# Run directly (no clone needed, requires Go 1.22+)
go run github.com/oadp-rebasebot/oadp-rebase/tools/prow-merge-bot-configs/tui@latest

# Or build from source
cd tui && go build -o prow-audit-tui .
```

```bash
./prow-audit-tui                           # interactive TUI
./prow-audit-tui --format text             # text output
./prow-audit-tui --format json             # JSON output
./prow-audit-tui --local /path/to/release  # use local checkout
```

Features:
- **4-tab interface**: Config Audit, Merge Queue, Field Compare, Tide Branches
- **Merge queue analysis**: detects tideErrLoopBlocker PRs ([prow#134](https://github.com/kubernetes-sigs/prow/issues/134)) that cause Tide error loops when `enforce_admins=true`
- **Clickable interface**: `[m]` buttons, `🌐` globe links to config files with line numbers, double-click to open in browser
- **Colorblind-friendly**: blue/orange/magenta/cyan palette with distinct icons for all statuses
- **Collapsible sections**: groups, repos, and individual PRs

See [`tui/README.md`](tui/README.md) for full keyboard/mouse reference.

Requires Go 1.22+ and optionally `gh` CLI for merge queue checks.

#### Bash Script (`audit.sh`)

The original bash audit script. Still works for simple text/markdown output and interactive mode (bash 4+ required for interactive).

```
./audit.sh [--branch main] [--format text|markdown|interactive] [--skip-queue] [--local <path>]
```

Both tools categorize repos into three groups with different expected configurations:
- **upstream-rebase**: Forks managed by rebasebot (expect `allow_force_pushes`, no `enforce_admins`)
- **oadp-owned-openshift**: OADP-maintained repos in `openshift/` org (expect `enforce_admins`, review count)
- **oadp-owned-migtools**: OADP-maintained repos in `migtools/` org (expect `enforce_admins`, review count)

### Relevant Documentation

- [Prow Approve Plugin & Approvers](https://docs.prow.k8s.io/docs/components/plugins/approve/approvers/)
- [Approve Go type](https://pkg.go.dev/sigs.k8s.io/prow/pkg/plugins#Approve)
- [Lgtm Go type](https://pkg.go.dev/sigs.k8s.io/prow/pkg/plugins#Lgtm)
- [Tide documentation](https://docs.prow.k8s.io/docs/components/core-components/tide/)

### openshift/ vs migtools/ Plugin List Difference

`openshift/` org repos have minimal plugin lists (typically just `- approve`) because they inherit most plugins from the **org-level** Prow config for the `openshift` org.

`migtools/` repos list all plugins explicitly (assign, blunderbuss, hold, label, lgtm, lifecycle, trigger, wip, approve, etc.) because `migtools` doesn't have org-level plugin inheritance and must specify everything per-repo. These repos also explicitly configure `external_plugins` for services like cherrypick, needs-rebase, jira-lifecycle-plugin, etc.
