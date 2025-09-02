#!/bin/bash
set -euo pipefail

DOWNSTREAM_BRANCH="oadp-1.5"
DOWNSTREAM_MODULE="github.com/openshift/velero"
UPSTREAM_MODULE="github.com/vmware-tanzu/velero"

GO_MOD_FILE="go.mod"
REPLACE_LINE="replace $UPSTREAM_MODULE => $DOWNSTREAM_MODULE $DOWNSTREAM_BRANCH"

# Replace existing line or append if not present
if grep -q "^replace $UPSTREAM_MODULE" "$GO_MOD_FILE"; then
    sed -i "s|^replace $UPSTREAM_MODULE.*|$REPLACE_LINE|" "$GO_MOD_FILE"
else
    echo "$REPLACE_LINE" >> "$GO_MOD_FILE"
fi

