#!/bin/bash
set -euo pipefail

# Generated from versions/oadp-1.3.env — do not edit manually.
# Run: make generate

DOWNSTREAM_BRANCH="oadp-1.3"
DOWNSTREAM_MODULE="github.com/openshift/velero"
GO_MOD_FILE="go.mod"

# Detect which velero module path the project uses (vmware-tanzu or velero-io)
if grep -qE "require.*github\.com/velero-io/velero " "$GO_MOD_FILE" || \
   grep -qE "^\s+github\.com/velero-io/velero " "$GO_MOD_FILE"; then
    UPSTREAM_MODULE="github.com/velero-io/velero"
else
    UPSTREAM_MODULE="github.com/vmware-tanzu/velero"
fi

# --- Step 1: go mod edit (must run BEFORE the sed replace) ---
# go mod edit validates the entire go.mod. The sed replace in step 2 writes a
# branch ref (e.g. "oadp-1.6") which is not valid semver. If go mod edit ran
# after sed, it would reject the file. So we run it first while the existing
# replace still has a valid pseudo-version.

# Update the require version to match the upstream tag from the SSOT.
go mod edit -require="${UPSTREAM_MODULE}@v1.12.4"

# Exclude kcp monorepo broken pseudo-version.
go mod edit -exclude=github.com/kcp-dev/kcp/sdk@v0.0.0-00010101000000-000000000000

# --- Step 2: sed replace (writes branch ref, resolved later by go mod tidy) ---
REPLACE_LINE="replace $UPSTREAM_MODULE => $DOWNSTREAM_MODULE $DOWNSTREAM_BRANCH"

# Remove any stale replace for the other module path
if [ "$UPSTREAM_MODULE" = "github.com/velero-io/velero" ]; then
    sed -i '/^replace github\.com\/vmware-tanzu\/velero /d' "$GO_MOD_FILE"
else
    sed -i '/^replace github\.com\/velero-io\/velero /d' "$GO_MOD_FILE"
fi

# Replace existing line or append if not present
if grep -q "^replace $UPSTREAM_MODULE " "$GO_MOD_FILE"; then
    sed -i "s|^replace $UPSTREAM_MODULE .*|$REPLACE_LINE|" "$GO_MOD_FILE"
else
    echo "$REPLACE_LINE" >> "$GO_MOD_FILE"
fi

# --- Step 3: handle velero/pkg/apis sub-module ---
# Upstream velero split pkg/apis into its own Go sub-module (June 2026, after
# v1.16.0). The sub-module only exists on main — not in any released tag yet.
# Only add the replace if the consumer's go.mod already references pkg/apis;
# adding it unconditionally breaks release branches where the sub-module's
# go.mod does not exist.
if grep -qE "(vmware-tanzu|velero-io)/velero/pkg/apis" "$GO_MOD_FILE"; then
    APIS_UPSTREAM="${UPSTREAM_MODULE}/pkg/apis"
    APIS_DOWNSTREAM="${DOWNSTREAM_MODULE}/pkg/apis"
    APIS_REPLACE_LINE="replace $APIS_UPSTREAM => $APIS_DOWNSTREAM $DOWNSTREAM_BRANCH"
    if grep -q "^replace $APIS_UPSTREAM " "$GO_MOD_FILE"; then
        sed -i "s|^replace $APIS_UPSTREAM .*|$APIS_REPLACE_LINE|" "$GO_MOD_FILE"
    else
        echo "$APIS_REPLACE_LINE" >> "$GO_MOD_FILE"
    fi
fi
