#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_DIR="$REPO_ROOT/versions"
OUTPUT_FILE="$REPO_ROOT/docs/version-matrix.md"
REPOS_YAML="$REPO_ROOT/repos.yaml"

command -v yq >/dev/null 2>&1 || { echo "Error: yq is required (https://github.com/mikefarah/yq)" >&2; exit 1; }

versions=()
for f in "$VERSIONS_DIR"/oadp-1.*.env; do
    [ -f "$f" ] || continue
    versions+=("$f")
done

if [ ${#versions[@]} -eq 0 ]; then
    echo "No versions files found in $VERSIONS_DIR" >&2
    exit 1
fi

# Collect release branch names from versions files
release_branches=()
for f in "${versions[@]}"; do
    unset OADP_BRANCH
    . "$f"
    release_branches+=("$OADP_BRANCH")
done

# Branch comparison functions (same logic as run-oadp-rebase.sh)
_ver_num() { echo "$1" | sed -n 's/oadp-1\.\([0-9]*\)/\1/p'; }

_branch_ge() {
    case "$1" in oadp-dev|main) return 0 ;; esac
    local cur min
    cur=$(_ver_num "$1"); min=$(_ver_num "$2")
    [ -z "$cur" ] && return 0; [ -z "$min" ] && return 0
    [ "$cur" -ge "$min" ]
}

_branch_le() {
    case "$1" in oadp-dev|main) return 0 ;; esac
    local cur max
    cur=$(_ver_num "$1"); max=$(_ver_num "$2")
    [ -z "$cur" ] && return 0; [ -z "$max" ] && return 0
    [ "$cur" -le "$max" ]
}

_repo_active() {
    local branch="$1" main_only="$2" min_b="$3" max_b="$4"
    if [ "$main_only" = "true" ] && [ "$branch" != "oadp-dev" ] && [ "$branch" != "main" ]; then
        return 1
    fi
    [ "$min_b" != "_NONE_" ] && { _branch_ge "$branch" "$min_b" || return 1; }
    [ "$max_b" != "_NONE_" ] && { _branch_le "$branch" "$max_b" || return 1; }
    return 0
}

# Read all repo data once from repos.yaml.
# Use "_NONE_" as sentinel for empty fields (POSIX read collapses consecutive tabs).
repo_data=$(yq -r '.repos[] | [.org, .repo, .wave, (.main_only // false), (.min_branch // "_NONE_"), (.max_branch // "_NONE_")] | @tsv' "$REPOS_YAML")
wave_nums=$(yq -r '.waves | keys | .[]' "$REPOS_YAML" | sort -n)

tmp_output="$(mktemp)"
trap 'rm -f "$tmp_output"' EXIT

{
cat <<'HEADER'
<!-- Auto-generated from versions/*.env and repos.yaml — do not edit manually. Run tools/generate-version-matrix.sh -->

# OADP Rebase Version Matrix

> **Auto-generated** from `versions/*.env` and `repos.yaml`. Do not edit manually — run `make generate` instead.

This document maps OADP versions to their upstream dependencies, tracks which repositories have downstream branches per version, and defines wave composition for the rebase process.

## Upstream Version Mapping

Each OADP release rebases against a specific set of upstream refs (tags or branches):

| OADP | Velero | Kopia | AWS Plugin | GCP Plugin | Azure Plugin | CSI Plugin | KubeVirt Plugin | Filebrowser |
|------|--------|-------|------------|------------|--------------|------------|-----------------|-------------|
HEADER

for f in "${versions[@]}"; do
    unset OADP_BRANCH VELERO_UPSTREAM_TAG KOPIA_UPSTREAM_TAG AWS_PLUGIN_TAG GCP_PLUGIN_TAG AZURE_PLUGIN_TAG CSI_PLUGIN_TAG KUBEVIRT_PLUGIN_REF FILEBROWSER_TAG VELERO_TAG_SHA
    . "$f"
    ver="${OADP_BRANCH#oadp-}"
    echo "| $ver | $VELERO_UPSTREAM_TAG | $KOPIA_UPSTREAM_TAG | $AWS_PLUGIN_TAG | $GCP_PLUGIN_TAG | $AZURE_PLUGIN_TAG | ${CSI_PLUGIN_TAG:-n/a} | $KUBEVIRT_PLUGIN_REF | ${FILEBROWSER_TAG:-n/a} |"
done

cat <<'MID1'

The CSI plugin was merged into Velero core after v1.12, so OADP 1.4+ no longer has a separate CSI plugin repository.

## Velero Tag SHAs

Used by `verify-tag-sha` pre-rebase hooks to detect silent tag recreation (supply chain risk):

| Velero Tag | Commit SHA |
|------------|------------|
MID1

for f in "${versions[@]}"; do
    unset VELERO_UPSTREAM_TAG VELERO_TAG_SHA
    . "$f"
    echo "| $VELERO_UPSTREAM_TAG | \`$VELERO_TAG_SHA\` |"
done

# --- Branch Coverage Matrix (derived from repos.yaml) ---

echo ""
echo "## Branch Coverage Matrix"
echo ""
echo "Which repositories have downstream branches per OADP version:"
echo ""

# Header row
printf "| Repository |"
for b in "${release_branches[@]}"; do printf " %s |" "${b#oadp-}"; done
printf " dev |\n"

# Separator row
printf '%s' "|------------|"
for _ in "${release_branches[@]}"; do printf '%s' "-----|"; done
printf '%s' "-----|"
echo ""

# One row per repo
while IFS=$'\t' read -r org repo wave main_only min_b max_b; do
    [ -z "$org" ] && continue
    printf "| %s/%s |" "$org" "$repo"
    for b in "${release_branches[@]}"; do
        if _repo_active "$b" "$main_only" "$min_b" "$max_b"; then
            printf " Y |"
        else
            printf " - |"
        fi
    done
    if [ "$main_only" = "true" ]; then
        printf " main |"
    else
        printf " Y |"
    fi
    echo ""
done <<< "$repo_data"

# --- Wave Composition (derived from repos.yaml) ---

echo ""
echo "## Wave Composition"
echo ""
echo "Each wave groups repositories that can be rebased in parallel. Waves must be executed sequentially since later waves depend on earlier ones."

all_branches=("${release_branches[@]}" "oadp-dev")

for branch in "${all_branches[@]}"; do
    if [ "$branch" = "oadp-dev" ]; then
        echo ""
        echo "### oadp-dev"
    else
        echo ""
        echo "### OADP ${branch#oadp-}"
    fi
    echo ""
    echo "| Wave | Repositories |"
    echo "|------|-------------|"

    for wave in $wave_nums; do
        repos_in_wave=""
        while IFS=$'\t' read -r _org repo w main_only min_b max_b; do
            [ -z "$_org" ] && continue
            [ "$w" != "$wave" ] && continue
            _repo_active "$branch" "$main_only" "$min_b" "$max_b" || continue
            if [ -z "$repos_in_wave" ]; then
                repos_in_wave="$repo"
            else
                repos_in_wave="$repos_in_wave, $repo"
            fi
        done <<< "$repo_data"
        [ -n "$repos_in_wave" ] && echo "| $wave | $repos_in_wave |"
    done
done

} > "$tmp_output"

mv "$tmp_output" "$OUTPUT_FILE"
trap - EXIT

echo "Generated $OUTPUT_FILE"
