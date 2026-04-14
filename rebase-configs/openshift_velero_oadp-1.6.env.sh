# ===============================================================
# Velero rebase configuration for OADP 1.6
# Uses upstream Velero release-1.18 branch
# ===============================================================

UPSTREAM_VELERO_BRANCH="release-1.18"
DESTINATION_DOWNSTREAM_VELERO_BRANCH="oadp-1.6"

SOURCE_UPSTREAM_REPO="https://github.com/vmware-tanzu/velero:$UPSTREAM_VELERO_BRANCH"
DESTINATION_DOWNSTREAM_REPO="openshift/velero:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/fix-malformed-filenames-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/restic-submodule-and-commit_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
