#!/bin/bash
set -euo pipefail

# OADP 1.3 uses project-velero/kopia directly (no OADP-maintained fork).
DOWNSTREAM_TAG="v0.13.0-velero.1"
DOWNSTREAM_MODULE="github.com/project-velero/kopia"
UPSTREAM_MODULE="github.com/kopia/kopia"

GO_MOD_FILE="go.mod"
REPLACE_LINE="replace $UPSTREAM_MODULE => $DOWNSTREAM_MODULE $DOWNSTREAM_TAG"

# Replace existing line or append if not present
if grep -q "^replace $UPSTREAM_MODULE" "$GO_MOD_FILE"; then
    sed -i "s|^replace $UPSTREAM_MODULE.*|$REPLACE_LINE|" "$GO_MOD_FILE"
else
    echo "$REPLACE_LINE" >> "$GO_MOD_FILE"
fi
