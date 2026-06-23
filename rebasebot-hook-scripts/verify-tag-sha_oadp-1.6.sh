#!/bin/bash

set -e
set -o pipefail

# Verify that the upstream Velero tag still points to the expected commit.
# Detects silent tag recreation (a supply chain risk).

EXPECTED_TAG="v1.18.1"
EXPECTED_SHA="26ef8fa7df68dfa9ab8aa4669b8ac47340b0c510"
UPSTREAM_REPO="https://github.com/velero-io/velero"

if [ "$REBASEBOT_SOURCE" != "$EXPECTED_TAG" ]; then
    echo "Source tag '$REBASEBOT_SOURCE' does not match expected tag '$EXPECTED_TAG'" >&2
    echo "Update this script if the upstream tag has changed." >&2
    exit 1
fi

LS_REMOTE_OUTPUT=$(git ls-remote "$UPSTREAM_REPO" "refs/tags/$EXPECTED_TAG" "refs/tags/$EXPECTED_TAG^{}")

if [ -z "$LS_REMOTE_OUTPUT" ]; then
    echo "Failed to resolve tag '$EXPECTED_TAG' from $UPSTREAM_REPO" >&2
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
    echo "TAG SHA MISMATCH for $EXPECTED_TAG" >&2
    echo "  Expected: $EXPECTED_SHA" >&2
    echo "  Resolved: $RESOLVED_SHA" >&2
    echo "The tag may have been recreated. Aborting rebase." >&2
    exit 1
fi

echo "Tag $EXPECTED_TAG verified: $RESOLVED_SHA"
