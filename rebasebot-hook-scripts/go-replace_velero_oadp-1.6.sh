#!/bin/bash
set -euo pipefail

# Generated from versions/oadp-1.6.env — do not edit manually.
# Run: make generate

DOWNSTREAM_BRANCH="oadp-1.6"
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
go mod edit -require="${UPSTREAM_MODULE}@v1.18.3-rc.1"

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


# --- Step 3: strip stale velero/pkg/apis sub-module references ---
# v1.18.x (and earlier) does NOT have the pkg/apis sub-module — that
# sub-module only exists on velero main. Strip any references left over
# from when the sub-module was manually carried on the openshift/velero
# release branch, so that go mod tidy does not try to resolve a non-existent
# sub-module go.mod.
# Use sed (not go mod edit -droprequire) — droprequire requires @version and
# fails silently without it.
sed -i '/^\s*github\.com\/vmware-tanzu\/velero\/pkg\/apis /d' "$GO_MOD_FILE"
sed -i '/^\s*github\.com\/velero-io\/velero\/pkg\/apis /d' "$GO_MOD_FILE"
sed -i '/^replace github\.com\/vmware-tanzu\/velero\/pkg\/apis /d' "$GO_MOD_FILE"
sed -i '/^replace github\.com\/velero-io\/velero\/pkg\/apis /d' "$GO_MOD_FILE"
