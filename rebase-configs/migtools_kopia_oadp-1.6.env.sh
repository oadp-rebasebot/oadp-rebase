# ===============================================================
# Kopia rebase configuration for OADP 1.6
# Uses upstream project-velero/kopia tag v0.22.3-velero-patch
# which corresponds to the Kopia version used by Velero release-1.18
# ===============================================================

UPSTREAM_KOPIA_REPO="project-velero/kopia"
UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO="v0.22.3-velero-patch"
DESTINATION_DOWNSTREAM_BRANCH="oadp-1.6"

SOURCE_UPSTREAM_REPO="https://github.com/$UPSTREAM_KOPIA_REPO:$UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:$DESTINATION_DOWNSTREAM_BRANCH"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-$DESTINATION_DOWNSTREAM_BRANCH"
