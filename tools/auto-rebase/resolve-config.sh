#!/bin/bash
set -euo pipefail

# Resolves a rebase target to its config variables.
# Outputs eval-able variable assignments to stdout.
#
# Usage: eval "$(resolve-config.sh <target>)"

SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

if [ $# -ne 1 ]; then
    echo "Usage: resolve-config.sh <target>" >&2
    exit 2
fi

TARGET="$1"

# Resolve config name using run-oadp-rebase.sh's get_config_name
config_name=$(grep -E "^[[:space:]]+${TARGET}\)" "$SCRIPT_DIR/run-oadp-rebase.sh" | head -1 | sed 's/.*echo "\(.*\)".*/\1/' || true)
if [ -z "$config_name" ]; then
    echo "Could not resolve config for target: ${TARGET}" >&2
    exit 1
fi

config_file="$SCRIPT_DIR/rebase-configs/${config_name}.env.sh"
if [ ! -f "$config_file" ]; then
    echo "Config file not found: ${config_file}" >&2
    exit 1
fi

# Load SSOT versions file if available (provides $VELERO_UPSTREAM_TAG etc.)
oadp_version=$(echo "$TARGET" | grep -oE 'oadp-[a-z0-9.]+' | head -1)
if [ -n "$oadp_version" ]; then
    versions_file="$SCRIPT_DIR/versions/${oadp_version}.env"
    [ -f "$versions_file" ] && . "$versions_file"
fi

# Load config vars
. "$config_file"

# Parse components
upstream_stripped=$(echo "$SOURCE_UPSTREAM_REPO" | sed 's|https://github.com/||')
upstream_repo=$(echo "$upstream_stripped" | cut -d: -f1)
upstream_ref=$(echo "$upstream_stripped" | cut -d: -f2)
dest_repo=$(echo "$DESTINATION_DOWNSTREAM_REPO" | cut -d: -f1)
dest_branch=$(echo "$DESTINATION_DOWNSTREAM_REPO" | cut -d: -f2)
rebase_repo_slug=$(echo "$REBASE_REPO" | cut -d: -f1)
rebase_branch=$(echo "$REBASE_REPO" | cut -d: -f2)
repo_name=$(echo "$TARGET" | sed -E 's/-(oadp-dev|oadp-1\.[0-9]+|main)$//')

# Output eval-able assignments
cat <<EOF
CONFIG_NAME="${config_name}"
REPO_NAME="${repo_name}"
DEST_REPO="${dest_repo}"
DEST_BRANCH="${dest_branch}"
REBASE_REPO_SLUG="${rebase_repo_slug}"
REBASE_BRANCH="${rebase_branch}"
UPSTREAM_REPO="${upstream_repo}"
UPSTREAM_REF="${upstream_ref}"
EOF
