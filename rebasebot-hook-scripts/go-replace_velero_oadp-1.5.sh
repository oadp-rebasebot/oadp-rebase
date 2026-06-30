#!/bin/bash
set -euo pipefail

# Generated from versions/oadp-1.5.env — do not edit manually.
# Run: make generate

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

# Update the require version to match the upstream tag from the SSOT.
go mod edit -require="${UPSTREAM_MODULE}@v1.16.0-rc.1"

# Exclude kcp monorepo broken pseudo-version.
# kcp-dev/kcp/cli's go.mod uses "replace kcp/sdk => ./sdk" which produces
# v0.0.0-00010101000000-000000000000 — a pseudo-version that doesn't exist
# on the module proxy. This causes go mod tidy to fail in consumers.
# See: https://github.com/kcp-dev/kcp/blob/cli/v0.27.1/cli/go.mod
go mod edit -exclude=github.com/kcp-dev/kcp/sdk@v0.0.0-00010101000000-000000000000
