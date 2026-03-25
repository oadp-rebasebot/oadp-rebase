SOURCE_UPSTREAM_REPO="https://github.com/kubevirt/kubevirt-velero-plugin:main"
DESTINATION_DOWNSTREAM_REPO="migtools/kubevirt-velero-plugin:oadp-1.6"
REBASE_REPO="oadp-rebasebot/kubevirt-velero-plugin:rebase-bot-oadp-1.6"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  "
