#!/bin/bash
set -euo pipefail

DOWNSTREAM_BRANCH="oadp-1.5"
DOWNSTREAM_MODULE="github.com/openshift/velero"
GO_MOD_FILE="go.mod"

# Detect which velero module path the project uses (vmware-tanzu or velero-io)
if grep -qE "require.*github\.com/velero-io/velero " "$GO_MOD_FILE" || \
   grep -qE "^\s+github\.com/velero-io/velero " "$GO_MOD_FILE"; then
    UPSTREAM_MODULE="github.com/velero-io/velero"
else
    UPSTREAM_MODULE="github.com/vmware-tanzu/velero"
fi

REPLACE_LINE="replace $UPSTREAM_MODULE => $DOWNSTREAM_MODULE $DOWNSTREAM_BRANCH"

# Update require entries (both single-line and block forms) to an upstream Velero tag.
# If not provided via env, preserve current upstream-style tag from go.mod.
VELERO_REQUIRE_VERSION="${VELERO_REQUIRE_VERSION:-$(
    sed -nE "s|^[[:space:]]*(require[[:space:]]+)?$UPSTREAM_MODULE[[:space:]]+([^[:space:]]+).*$|\\2|p" "$GO_MOD_FILE" | head -n1
)}"
if [[ "$VELERO_REQUIRE_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
    sed -Ei "s|^([[:space:]]*(require[[:space:]]+)?$UPSTREAM_MODULE)[[:space:]]+[^[:space:]]+|\\1 $VELERO_REQUIRE_VERSION|" "$GO_MOD_FILE"
else
    echo "Skipping Velero require rewrite: set VELERO_REQUIRE_VERSION to an upstream tag (e.g. v1.18.1 or v1.18.1-rc.2)" >&2
fi

# Remove any stale replace for the other module path
if [ "$UPSTREAM_MODULE" = "github.com/velero-io/velero" ]; then
    sed -i '/^replace github\.com\/vmware-tanzu\/velero /d' "$GO_MOD_FILE"
else
    sed -i '/^replace github\.com\/velero-io\/velero /d' "$GO_MOD_FILE"
fi

# Replace existing line or append if not present
if grep -q "^replace $UPSTREAM_MODULE" "$GO_MOD_FILE"; then
    sed -i "s|^replace $UPSTREAM_MODULE.*|$REPLACE_LINE|" "$GO_MOD_FILE"
else
    echo "$REPLACE_LINE" >> "$GO_MOD_FILE"
fi
