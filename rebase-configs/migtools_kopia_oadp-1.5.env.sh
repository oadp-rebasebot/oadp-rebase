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
# Fetch the latest minor Velero release for a given major version,
# extract the Kopia commit hash from its go.mod, and determine the
# corresponding Kopia tag in the upstream project-velero/kopia repo.
# This ensures that the Kopia rebase aligns with the Velero rebase.
# ===============================================================

UPSTREAM_VELERO_MAJOR_VERSION=v1.16
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-1.5

tags_body=$(fetch_github_api "https://api.github.com/repos/vmware-tanzu/velero/tags?per_page=100") || exit 1

LATEST_VELERO_TAG=$(
  printf '%s\n' "$tags_body" \
  | grep -Eo "\"name\": \"${UPSTREAM_VELERO_MAJOR_VERSION}\.[0-9]+\"" \
  | awk -F'"' '{print $4}' \
  | sort -V \
  | tail -n 1
)

KOPIA_HASH=$(curl -s -L "https://raw.githubusercontent.com/vmware-tanzu/velero/$LATEST_VELERO_TAG/go.mod" \
  | grep 'replace github.com/kopia/kopia' \
  | awk '{print $NF}' \
  | awk -F'-' '{print $NF}')

UPSTREAM_KOPIA_REPO="project-velero/kopia"

tags_body=$(fetch_github_api "https://api.github.com/repos/${UPSTREAM_KOPIA_REPO}/tags") || exit 1

UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO=$(printf '%s\n' "$tags_body" \
    | grep -E '"name":|"sha":' \
    | paste - - \
    | grep "$KOPIA_HASH" \
    | awk -F'"' '{print $4}' \
    | head -n1
)

SOURCE_UPSTREAM_REPO="https://github.com/$UPSTREAM_KOPIA_REPO:$UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

