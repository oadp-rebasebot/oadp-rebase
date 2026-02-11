# function to fetch API body or fail on error
# This is to ensure we can handle e.g. 403 "API rate limit exceeded" error
fetch_github_api() {
    url="$1"

    response=$(curl -s -L -w "%{http_code}" "$url")
    http_code=$(printf "%s" "$response" | tail -c 3)
    body=$(printf "%s" "$response" | head -c $(($(printf "%s" "$response" | wc -c) - 3)))

    if [ "$http_code" != "200" ]; then
        printf 'Error: API returned %s\n' "$http_code" >&2
        printf '%s\n' "$body" >&2
        return 1
    fi

    printf '%s\n' "$body"
}

# ===============================================================
# Determine the correct AWS plugin version to rebase based on the
# Velero major version. Uses the compatibility matrix from the plugin
# README to map Velero version to plugin version, then finds the
# latest tag in the corresponding plugin release branch.
# ===============================================================

UPSTREAM_VELERO_MAJOR_VERSION=v1.18
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-1.6

UPSTREAM_PLUGIN_REPO="vmware-tanzu/velero-plugin-for-aws"

# Fetch the AWS plugin compatibility matrix from README
README_URL="https://raw.githubusercontent.com/${UPSTREAM_PLUGIN_REPO}/refs/heads/main/README.md"
README_CONTENT=$(curl -s -L "$README_URL")

# Extract plugin version for our Velero version from the compatibility matrix
# The matrix format is: | v1.12.x | v1.16.x |
UPSTREAM_PLUGIN_VERSION=$(printf '%s\n' "$README_CONTENT" \
    | grep -E "^\| v[0-9]+\.[0-9]+\.x.*\| ${UPSTREAM_VELERO_MAJOR_VERSION}\.x" \
    | sed -E 's/^\|\s*v([0-9]+\.[0-9]+)\.x.*/\1/' \
    | head -n1)

if [ -z "$UPSTREAM_PLUGIN_VERSION" ]; then
    printf 'Error: No match for Velero %s in AWS plugin compatibility matrix\n' "$UPSTREAM_VELERO_MAJOR_VERSION" >&2
    printf 'Compatibility matrix:\n' >&2
    printf '%s\n' "$README_CONTENT" | grep -A 10 "## Compatibility" >&2
    exit 1
fi

# Fetch the latest tag in the plugin release series
tags_body=$(fetch_github_api "https://api.github.com/repos/${UPSTREAM_PLUGIN_REPO}/tags?per_page=100") || exit 1

UPSTREAM_PLUGIN_TAG_BRANCH_FOR_VELERO=$(
    printf '%s\n' "$tags_body" \
    | grep -Eo "\"name\": \"v${UPSTREAM_PLUGIN_VERSION}\.[0-9]+\"" \
    | awk -F'"' '{print $4}' \
    | sort -V \
    | tail -n1
)

if [ -z "$UPSTREAM_PLUGIN_TAG_BRANCH_FOR_VELERO" ]; then
    # Fall back to release branch if no tag found
    UPSTREAM_PLUGIN_TAG_BRANCH_FOR_VELERO="release-${UPSTREAM_PLUGIN_VERSION}"
    printf 'Warning: No release tag found for v%s, using branch %s\n' "$UPSTREAM_PLUGIN_VERSION" "$UPSTREAM_PLUGIN_TAG_BRANCH_FOR_VELERO" >&2
fi

SOURCE_UPSTREAM_REPO="https://github.com/${UPSTREAM_PLUGIN_REPO}:${UPSTREAM_PLUGIN_TAG_BRANCH_FOR_VELERO}"
DESTINATION_DOWNSTREAM_REPO="openshift/velero-plugin-for-aws:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero-plugin-for-aws:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  "
