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

RESOLVED_SHA=$(git ls-remote "$UPSTREAM_REPO" "$EXPECTED_TAG" | awk '{print $1}')

if [ -z "$RESOLVED_SHA" ]; then
    echo "Failed to resolve tag '$EXPECTED_TAG' from $UPSTREAM_REPO" >&2
    exit 1
fi

# Handle annotated tags — dereference to the commit
DEREF_SHA=$(git ls-remote "$UPSTREAM_REPO" "$EXPECTED_TAG^{}" | awk '{print $1}')
if [ -n "$DEREF_SHA" ]; then
    RESOLVED_SHA="$DEREF_SHA"
fi

if [ "$RESOLVED_SHA" != "$EXPECTED_SHA" ]; then
    echo "TAG SHA MISMATCH for $EXPECTED_TAG" >&2
    echo "  Expected: $EXPECTED_SHA" >&2
    echo "  Resolved: $RESOLVED_SHA" >&2
    echo "The tag may have been recreated. Aborting rebase." >&2
    exit 1
fi

echo "Tag $EXPECTED_TAG verified: $RESOLVED_SHA"
