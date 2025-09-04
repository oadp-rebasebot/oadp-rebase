# ===============================================================
# Fetch the latest minor Velero release for a given major version.
# ===============================================================
UPSTREAM_VELERO_BRANCH=main
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-dev

SOURCE_UPSTREAM_REPO="https://github.com/vmware-tanzu/velero:$UPSTREAM_VELERO_BRANCH"
DESTINATION_DOWNSTREAM_REPO="openshift/velero:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/restic-submodule-and-commit_oadp-dev.sh \
  "
