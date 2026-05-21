# ===============================================================
# HyperShift OADP Plugin rebase configuration for main (oadp-dev)
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero dependencies at oadp-dev.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/openshift/hypershift-oadp-plugin:main"
DESTINATION_DOWNSTREAM_REPO="openshift/hypershift-oadp-plugin:main"
REBASE_REPO="oadp-rebasebot/hypershift-oadp-plugin:rebase-bot-main"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
