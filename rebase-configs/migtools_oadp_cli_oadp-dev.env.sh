# ===============================================================
# OADP CLI rebase configuration for oadp-dev
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod
# to point velero, kopia, and oadp-non-admin dependencies at oadp-dev.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/migtools/oadp-cli:oadp-dev"
DESTINATION_DOWNSTREAM_REPO="migtools/oadp-cli:oadp-dev"
REBASE_REPO="oadp-rebasebot/oadp-cli:rebase-bot-oadp-dev"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-non-admin_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-use-tag_oadp-operator_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
