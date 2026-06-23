# ===============================================================
# OADP Must-Gather rebase configuration for oadp-dev
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero, oadp-operator, and oadp-non-admin dependencies
# at their respective oadp-dev branches.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/openshift/oadp-must-gather:oadp-dev"
DESTINATION_DOWNSTREAM_REPO="openshift/oadp-must-gather:oadp-dev"
REBASE_REPO="oadp-rebasebot/oadp-must-gather:rebase-bot-oadp-dev"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-operator_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-non-admin_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/velero-submodule-and-commit_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/restic-submodule-and-commit_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/kopia-submodule-and-commit_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
