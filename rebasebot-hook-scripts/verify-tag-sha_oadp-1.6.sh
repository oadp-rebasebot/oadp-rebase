#!/bin/bash

set -e
set -o pipefail

# Verify that the upstream Velero tag still points to the expected commit.
# Detects silent tag recreation (a supply chain risk).

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OADP_VERSION=$(basename "$0" | grep -oE 'oadp-1\.[0-9]+')
VERSIONS_FILE="$SCRIPT_DIR/../versions/${OADP_VERSION}.env"
if [ -f "$VERSIONS_FILE" ]; then
    . "$VERSIONS_FILE"
    EXPECTED_SHA="$VELERO_TAG_SHA"
else
    case "$OADP_VERSION" in
        oadp-1.3) EXPECTED_SHA="7d8417b2c58792422ed6dd4b4c0cf3848b0beb56" ;;
        oadp-1.4) EXPECTED_SHA="8afe3cea8b7058f7baaf447b9fb407312c40d2da" ;;
        oadp-1.5) EXPECTED_SHA="a60808256d36a77a42e1ebc160ee8117477a83fa" ;;
        oadp-1.6) EXPECTED_SHA="c253c7fe37d78c9b7e55c68544f7c5b2608712d8" ;;
        *) echo "Unknown OADP version: $OADP_VERSION" >&2; exit 1 ;;
    esac
fi
UPSTREAM_REPO="https://github.com/velero-io/velero"
TAG="$REBASEBOT_SOURCE"

LS_REMOTE_OUTPUT=$(git ls-remote "$UPSTREAM_REPO" "refs/tags/$TAG" "refs/tags/$TAG^{}")

if [ -z "$LS_REMOTE_OUTPUT" ]; then
    echo "Failed to resolve tag '$TAG' from $UPSTREAM_REPO" >&2
    exit 1
fi

COMMIT_SHA=$(echo "$LS_REMOTE_OUTPUT" | grep '\^{}' | cut -f1 || true)
TAG_OBJECT_SHA=$(echo "$LS_REMOTE_OUTPUT" | grep -v '\^{}' | cut -f1 || true)

# For annotated tags, the commit SHA comes from the dereferenced entry;
# for lightweight tags, the tag points directly to the commit
if [ -n "$COMMIT_SHA" ]; then
    RESOLVED_SHA="$COMMIT_SHA"
else
    RESOLVED_SHA="$TAG_OBJECT_SHA"
fi

if [ "$RESOLVED_SHA" != "$EXPECTED_SHA" ]; then
    echo "TAG SHA MISMATCH for $TAG" >&2
    echo "  Expected: $EXPECTED_SHA" >&2
    echo "  Resolved: $RESOLVED_SHA" >&2
    echo "The tag may have been recreated. Aborting rebase." >&2
    exit 1
fi

echo "Tag $TAG verified: $RESOLVED_SHA"
