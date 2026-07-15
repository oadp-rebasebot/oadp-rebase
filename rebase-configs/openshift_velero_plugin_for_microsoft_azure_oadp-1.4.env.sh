# ===============================================================
# Azure plugin rebase configuration for OADP 1.4
# Upstream tag sourced from versions/oadp-1.4.env
# which corresponds to the Azure plugin version for Velero 1.14
# ===============================================================

UPSTREAM_PLUGIN_REPO="velero-io/velero-plugin-for-microsoft-azure"
UPSTREAM_PLUGIN_TAG="${AZURE_PLUGIN_TAG:?Missing AZURE_PLUGIN_TAG from versions env}"
DESTINATION_DOWNSTREAM_BRANCH="oadp-1.4"

SOURCE_UPSTREAM_REPO="https://github.com/${UPSTREAM_PLUGIN_REPO}:${UPSTREAM_PLUGIN_TAG}"
DESTINATION_DOWNSTREAM_REPO="openshift/velero-plugin-for-microsoft-azure:$DESTINATION_DOWNSTREAM_BRANCH"
REBASE_REPO="oadp-rebasebot/velero-plugin-for-microsoft-azure:rebase-bot-$DESTINATION_DOWNSTREAM_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.4.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-reconcile-upstream.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
