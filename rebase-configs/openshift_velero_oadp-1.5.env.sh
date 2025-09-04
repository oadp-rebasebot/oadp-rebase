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
# Fetch the latest minor Velero release for a given major version.
# ===============================================================

UPSTREAM_VELERO_MAJOR_VERSION=v1.16
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-1.5

tags_body=$(fetch_github_api "https://api.github.com/repos/vmware-tanzu/velero/tags?per_page=100") || exit 1

# Do not include -rc.X versions, only released Velero
LATEST_VELERO_TAG=$(
  printf '%s\n' "$tags_body" \
  | grep -Eo "\"name\": \"${UPSTREAM_VELERO_MAJOR_VERSION}\.[0-9]+\"" \
  | awk -F'"' '{print $4}' \
  | sort -V \
  | tail -n 1
)

SOURCE_UPSTREAM_REPO="https://github.com/vmware-tanzu/velero:$LATEST_VELERO_TAG"
DESTINATION_DOWNSTREAM_REPO="openshift/velero:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-1.5.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/restic-submodule-and-commit_oadp-1.5.sh \
  "
