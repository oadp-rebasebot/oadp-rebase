# ===============================================================
# Velero rebase configuration for OADP 1.5
# Upstream tag sourced from versions/oadp-1.5.env
# ===============================================================

UPSTREAM_VELERO_BRANCH="$VELERO_UPSTREAM_TAG"
DESTINATION_DOWNSTREAM_VELERO_BRANCH="oadp-1.5"

SOURCE_UPSTREAM_REPO="https://github.com/velero-io/velero:$UPSTREAM_VELERO_BRANCH"
DESTINATION_DOWNSTREAM_REPO="openshift/velero:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--pre-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/verify-tag-sha_oadp-1.5.sh \
  --post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/fix-malformed-filenames-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-1.5.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/restic-submodule-and-commit_oadp-1.5.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
