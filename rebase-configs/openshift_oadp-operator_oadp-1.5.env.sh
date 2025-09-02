# The SOURCE and DESTINATION repo and branch are the same
# It is a hack to ensure we don't really rebase, but we run the hooks
SOURCE_UPSTREAM_REPO="https://github.com/openshift/oadp-operator:oadp-1.5"
DESTINATION_DOWNSTREAM_REPO="openshift/oadp-operator:oadp-1.5"
REBASE_REPO="oadp-rebasebot/oadp-operator:rebase-bot-oadp-1.5"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.5.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  "
