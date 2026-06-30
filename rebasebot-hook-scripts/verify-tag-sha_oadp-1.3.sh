#!/bin/bash

set -e
set -o pipefail

# Verify that the upstream Velero tag still points to the expected commit.
# Detects silent tag recreation (a supply chain risk).
#
# Generated from versions/oadp-1.3.env — do not edit manually.
# Run: make generate

EXPECTED_SHA="684f71306e9c2fda204a16cb012dc209523cfae1"
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
