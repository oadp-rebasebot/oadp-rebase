# ===============================================================
# OpenShift Velero Plugin rebase configuration for oadp-dev
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero dependency at the downstream oadp-dev branch.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/openshift/openshift-velero-plugin:oadp-dev"
DESTINATION_DOWNSTREAM_REPO="openshift/openshift-velero-plugin:oadp-dev"
REBASE_REPO="oadp-rebasebot/openshift-velero-plugin:rebase-bot-oadp-dev"

GO_VET_TAGS="exclude_graphdriver_devicemapper,exclude_graphdriver_btrfs"
EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
