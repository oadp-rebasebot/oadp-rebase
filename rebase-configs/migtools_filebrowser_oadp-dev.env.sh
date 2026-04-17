SOURCE_UPSTREAM_REPO="https://github.com/filebrowser/filebrowser:master"
DESTINATION_DOWNSTREAM_REPO="migtools/filebrowser:oadp-dev"
REBASE_REPO="oadp-rebasebot/filebrowser:rebase-bot-oadp-dev"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/filebrowser-regenerate-package-lock-and-commit.sh \
  "
