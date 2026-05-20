# ===============================================================
# AWS plugin rebase configuration for OADP 1.6
# Uses upstream velero-plugin-for-aws v1.14.1
# which corresponds to the AWS plugin version for Velero 1.18
# (v1.14.1 go.mod references github.com/velero-io/velero v1.18.x)
# ===============================================================

UPSTREAM_PLUGIN_REPO="velero-io/velero-plugin-for-aws"
UPSTREAM_PLUGIN_TAG="v1.14.1"
DESTINATION_DOWNSTREAM_BRANCH="oadp-1.6"

SOURCE_UPSTREAM_REPO="https://github.com/${UPSTREAM_PLUGIN_REPO}:${UPSTREAM_PLUGIN_TAG}"
DESTINATION_DOWNSTREAM_REPO="openshift/velero-plugin-for-aws:$DESTINATION_DOWNSTREAM_BRANCH"
REBASE_REPO="oadp-rebasebot/velero-plugin-for-aws:rebase-bot-$DESTINATION_DOWNSTREAM_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
