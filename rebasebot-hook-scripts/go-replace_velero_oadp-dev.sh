#!/bin/bash
set -euo pipefail

DOWNSTREAM_BRANCH="oadp-dev"
DOWNSTREAM_MODULE="github.com/openshift/velero"
GO_MOD_FILE="go.mod"

fetch_github_api() {
    local url="$1"
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$url"
    else
        curl -fsSL "$url"
    fi
}

fetch_all_velero_tags() {
    local page=1
    local combined=""
    while :; do
        local body
        if ! body="$(fetch_github_api "https://api.github.com/repos/velero-io/velero/tags?per_page=100&page=$page")"; then
            echo "Failed to fetch Velero tags from GitHub API (page $page)" >&2
            exit 1
        fi
        combined="${combined}"$'\n'"${body}"
        local count
        count="$(printf '%s\n' "$body" | grep -c "\"name\": \"" || true)"
        if [ "$count" -lt 100 ]; then
            break
        fi
        page=$((page + 1))
    done
    printf '%s\n' "$combined"
}

# Detect which velero module path the project uses (vmware-tanzu or velero-io)
if grep -qE "require.*github\.com/velero-io/velero " "$GO_MOD_FILE" || \
   grep -qE "^\s+github\.com/velero-io/velero " "$GO_MOD_FILE"; then
    UPSTREAM_MODULE="github.com/velero-io/velero"
else
    UPSTREAM_MODULE="github.com/vmware-tanzu/velero"
fi

REPLACE_LINE="replace $UPSTREAM_MODULE => $DOWNSTREAM_MODULE $DOWNSTREAM_BRANCH"

# Update require entries (both single-line and block forms) to the latest upstream Velero tag.
# Intentionally considers RC tags as valid upstream targets for oadp-dev alignment.
TAGS_BODY="$(fetch_all_velero_tags)"
UPSTREAM_VELERO_TAG="$(
    printf '%s\n' "$TAGS_BODY" \
    | grep -Eo "\"name\": \"v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?\"" \
    | awk -F'"' '{print $4}' \
    | sort -V \
    | tail -n1
)"
if [ -z "$UPSTREAM_VELERO_TAG" ]; then
    echo "Failed to determine latest upstream Velero tag" >&2
    exit 1
fi
sed -Ei "s|^([[:space:]]*(require[[:space:]]+)?$UPSTREAM_MODULE)[[:space:]]+[^[:space:]]+|\\1 $UPSTREAM_VELERO_TAG|" "$GO_MOD_FILE"

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
