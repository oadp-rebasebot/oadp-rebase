UPSTREAM_PLUGIN_BRANCH=main
DESTINATION_DOWNSTREAM_PLUGIN_BRANCH=oadp-dev

SOURCE_UPSTREAM_REPO="https://github.com/velero-io/velero-plugin-for-gcp:$UPSTREAM_PLUGIN_BRANCH"
DESTINATION_DOWNSTREAM_REPO="openshift/velero-plugin-for-gcp:$DESTINATION_DOWNSTREAM_PLUGIN_BRANCH"
REBASE_REPO="oadp-rebasebot/velero-plugin-for-gcp:rebase-bot-$DESTINATION_DOWNSTREAM_PLUGIN_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
