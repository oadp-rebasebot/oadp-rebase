# ===============================================================
# OADP VMDP (VM Data Protection) rebase configuration for oadp-dev
# Rebased from migtools/kopia (our downstream Kopia fork).
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/migtools/kopia:oadp-dev"
DESTINATION_DOWNSTREAM_REPO="migtools/oadp-vmdp:oadp-dev"
REBASE_REPO="oadp-rebasebot/oadp-vmdp:rebase-bot-oadp-dev"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-sync-kopia.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-reconcile-upstream.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
