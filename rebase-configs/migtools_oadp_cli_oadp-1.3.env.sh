# ===============================================================
# OADP CLI configuration for OADP 1.3
# Downstream-only repository (no upstream to rebase from).
# SOURCE and DESTINATION are the same; hooks update go.mod.
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/migtools/oadp-cli:oadp-1.3"
DESTINATION_DOWNSTREAM_REPO="migtools/oadp-cli:oadp-1.3"
REBASE_REPO="oadp-rebasebot/oadp-cli:rebase-bot-oadp-1.3"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.3.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_kopia_oadp-1.3.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
