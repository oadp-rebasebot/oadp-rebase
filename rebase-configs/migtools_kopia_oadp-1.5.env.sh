# ===============================================================
# Fetch the latest minor Velero release for a given major version,
# extract the Kopia commit hash from its go.mod, and determine the
# corresponding Kopia tag in the upstream project-velero/kopia repo.
# This ensures that the Kopia rebase aligns with the Velero rebase.
# ===============================================================

UPSTREAM_VELERO_MAJOR_VERSION=v1.16
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-1.5

LATEST_VELERO_TAG=$(
  curl -s -L "https://api.github.com/repos/vmware-tanzu/velero/tags?per_page=100" \
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

UPSTREAM_KOPIA_TAG_FOR_VELERO=$(curl -s -L "https://api.github.com/repos/${UPSTREAM_KOPIA_REPO}/tags" \
  | grep -E '"name":|"sha":' \
  | paste - - \
  | grep "$KOPIA_HASH" \
  | awk -F'"' '{print $4}' \
  | head -n1
)

SOURCE_UPSTREAM_REPO="https://github.com/$UPSTREAM_KOPIA_REPO:$UPSTREAM_KOPIA_TAG_FOR_VELERO"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

