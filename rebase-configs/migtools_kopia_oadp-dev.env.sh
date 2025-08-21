# ===============================================================
# Track the upstream Velero branch, extract the Kopia commit hash
# from its go.mod file, and determine the corresponding Kopia
# reference (tag or branch) in the upstream project-velero/kopia
# repository. This ensures that the Kopia rebase matches the
# Velero rebase.
# ===============================================================

UPSTREAM_VELERO_BRANCH=main
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-dev

KOPIA_HASH=$(curl -s -L "https://raw.githubusercontent.com/vmware-tanzu/velero/$UPSTREAM_VELERO_BRANCH/go.mod" \
  | grep 'replace github.com/kopia/kopia' \
  | awk '{print $NF}' \
  | awk -F'-' '{print $NF}')

UPSTREAM_KOPIA_REPO="project-velero/kopia"

UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO=$(curl -s -L "https://api.github.com/repos/${UPSTREAM_KOPIA_REPO}/tags" \
  | grep -E '"name":|"sha":' \
  | paste - - \
  | grep "$KOPIA_HASH" \
  | awk -F'"' '{print $4}' \
  | head -n1
)

if [ -z "$UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO" ]; then
  UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO=$(
    curl -s "https://api.github.com/repos/${UPSTREAM_KOPIA_REPO}/branches?per_page=100" \
    | grep -E '"name":|"sha":' \
    | paste - - \
    | grep "$KOPIA_HASH" \
    | awk -F'"' '{print $4}' \
    | head -n1
  )
fi

SOURCE_UPSTREAM_REPO="https://github.com/$UPSTREAM_KOPIA_REPO:$UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

