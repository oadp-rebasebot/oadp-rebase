SOURCE_UPSTREAM_REPO="https://github.com/kubevirt/kubevirt-velero-plugin:main"
DESTINATION_DOWNSTREAM_REPO="migtools/kubevirt-velero-plugin:main"
REBASE_REPO="oadp-rebasebot/kubevirt-velero-plugin:rebase-bot-main"

HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-dev.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "

EXTRA_REBASEBOT_ARGS="--always-run-hooks --exclude-commits bd418fff44a93192e7ef5dcf59f41a72893d69c2 e1aabe09b7484c27c5172a07acb1661368b81fb7"
