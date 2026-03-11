# ===============================================================
# KubeVirt Datamover Plugin rebase configuration for oadp-dev
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero dependency at oadp-dev.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/migtools/kubevirt-datamover-plugin:oadp-dev"
DESTINATION_DOWNSTREAM_REPO="migtools/kubevirt-datamover-plugin:oadp-dev"
REBASE_REPO="oadp-rebasebot/kubevirt-datamover-plugin:rebase-bot-oadp-dev"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_kubevirt-datamover-controller_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  "
