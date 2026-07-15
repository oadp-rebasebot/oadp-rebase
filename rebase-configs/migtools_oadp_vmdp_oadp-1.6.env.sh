# ===============================================================
# OADP VMDP (VM Data Protection) rebase configuration for OADP 1.6
# Rebased from migtools/kopia (our downstream Kopia fork).
# ===============================================================

SOURCE_UPSTREAM_REPO="https://github.com/migtools/kopia:oadp-1.6"
DESTINATION_DOWNSTREAM_REPO="migtools/oadp-vmdp:oadp-1.6"
REBASE_REPO="oadp-rebasebot/oadp-vmdp:rebase-bot-oadp-1.6"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-sync-kopia.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-reconcile-upstream.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
