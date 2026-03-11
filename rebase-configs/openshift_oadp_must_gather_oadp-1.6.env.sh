# ===============================================================
# OADP Must-Gather rebase configuration for OADP 1.6
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero, oadp-operator, and oadp-non-admin dependencies
# at their respective oadp-1.6 branches.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/openshift/oadp-must-gather:oadp-1.6"
DESTINATION_DOWNSTREAM_REPO="openshift/oadp-must-gather:oadp-1.6"
REBASE_REPO="oadp-rebasebot/oadp-must-gather:rebase-bot-oadp-1.6"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-operator_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-non-admin_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  "
