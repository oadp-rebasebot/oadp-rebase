# ===============================================================
# KubeVirt Velero Plugin rebase configuration for OADP 1.3
# Upstream tag sourced from versions/oadp-1.3.env
# ===============================================================

UPSTREAM_PLUGIN_REPO="kubevirt/kubevirt-velero-plugin"
UPSTREAM_PLUGIN_TAG="${KUBEVIRT_PLUGIN_TAG:?Missing KUBEVIRT_PLUGIN_TAG from versions env}"
DESTINATION_DOWNSTREAM_BRANCH="oadp-1.3"

SOURCE_UPSTREAM_REPO="https://github.com/${UPSTREAM_PLUGIN_REPO}:${UPSTREAM_PLUGIN_TAG}"
DESTINATION_DOWNSTREAM_REPO="migtools/kubevirt-velero-plugin:$DESTINATION_DOWNSTREAM_BRANCH"
REBASE_REPO="oadp-rebasebot/kubevirt-velero-plugin:rebase-bot-$DESTINATION_DOWNSTREAM_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.3.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-1.3.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
