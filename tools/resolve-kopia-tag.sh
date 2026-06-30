#!/bin/bash
set -euo pipefail

# Given a Velero tag, resolves the corresponding kopia tag/branch from
# the project-velero/kopia repo by matching the commit SHA in Velero's
# go.mod replace directive.
#
# Usage: resolve-kopia-tag.sh <velero-tag>
# Example: resolve-kopia-tag.sh v1.18.2-rc.2

if [ $# -ne 1 ]; then
    echo "Usage: resolve-kopia-tag.sh <velero-tag>" >&2
    exit 2
fi

VELERO_TAG="$1"
KOPIA_REPO="project-velero/kopia"

# Fetch Velero go.mod and extract kopia replace line
REPLACE_LINE=$(gh api "repos/velero-io/velero/contents/go.mod?ref=${VELERO_TAG}" \
    --jq '.content' | base64 -d | grep 'replace github.com/kopia/kopia' || true)

if [ -z "$REPLACE_LINE" ]; then
    echo "No kopia replace directive found in Velero ${VELERO_TAG} go.mod" >&2
    exit 1
fi

KOPIA_VERSION=$(echo "$REPLACE_LINE" | awk '{print $NF}')
echo "Velero ${VELERO_TAG} go.mod: ${KOPIA_VERSION}" >&2

# If it's already a real version (not pseudo), output directly
if [[ ! "$KOPIA_VERSION" =~ ^v0\.0\.0- ]]; then
    echo "$KOPIA_VERSION"
    exit 0
fi

# Pseudo-version — extract commit SHA prefix (last 12 chars)
COMMIT_PREFIX=$(echo "$KOPIA_VERSION" | awk -F'-' '{print $NF}')
echo "Resolving commit prefix: ${COMMIT_PREFIX}" >&2

# Search tags
TAG_MATCH=$(gh api "repos/${KOPIA_REPO}/tags?per_page=100" \
    --jq ".[] | select(.commit.sha | startswith(\"${COMMIT_PREFIX}\")) | .name" 2>/dev/null | head -1 || true)

if [ -n "$TAG_MATCH" ]; then
    echo "Matched tag: ${TAG_MATCH}" >&2
    echo "$TAG_MATCH"
    exit 0
fi

# Search branches
BRANCH_MATCH=$(gh api "repos/${KOPIA_REPO}/branches?per_page=100" \
    --jq ".[] | select(.commit.sha | startswith(\"${COMMIT_PREFIX}\")) | .name" 2>/dev/null | head -1 || true)

if [ -n "$BRANCH_MATCH" ]; then
    echo "Matched branch: ${BRANCH_MATCH}" >&2
    echo "$BRANCH_MATCH"
    exit 0
fi

echo "Could not resolve kopia ref for commit ${COMMIT_PREFIX}" >&2
exit 1
