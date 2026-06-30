# ===============================================================
# HyperShift OADP Plugin rebase configuration for OADP 1.5
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero dependencies at oadp-1.5.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/openshift/hypershift-oadp-plugin:oadp-1.5"
DESTINATION_DOWNSTREAM_REPO="openshift/hypershift-oadp-plugin:oadp-1.5"
REBASE_REPO="oadp-rebasebot/hypershift-oadp-plugin:rebase-bot-oadp-1.5"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.5.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
