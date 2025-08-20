#!/bin/bash
set -euo pipefail

DOWNSTREAM_BRANCH="oadp-dev"
DOWNSTREAM_MODULE="github.com/migtools/oadp-non-admin"

GO_MOD_FILE="go.mod"

if grep -q "^[[:space:]]*$DOWNSTREAM_MODULE " "$GO_MOD_FILE"; then
    sed -i "s|^\([[:space:]]*$DOWNSTREAM_MODULE\)[[:space:]]\+.*|\1 $DOWNSTREAM_BRANCH|" "$GO_MOD_FILE"
else
    echo "Module $DOWNSTREAM_MODULE not found in $GO_MOD_FILE"
    exit 1
fi

