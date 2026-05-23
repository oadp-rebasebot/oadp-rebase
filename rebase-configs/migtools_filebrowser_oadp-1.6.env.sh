# ===============================================================
# Filebrowser rebase configuration for OADP 1.6
# Uses upstream filebrowser v2.63.5
# ===============================================================

UPSTREAM_FILEBROWSER_REPO="filebrowser/filebrowser"
UPSTREAM_FILEBROWSER_TAG="v2.63.5"

SOURCE_UPSTREAM_REPO="https://github.com/${UPSTREAM_FILEBROWSER_REPO}:${UPSTREAM_FILEBROWSER_TAG}"
DESTINATION_DOWNSTREAM_REPO="migtools/filebrowser:oadp-1.6"
REBASE_REPO="oadp-rebasebot/filebrowser:rebase-bot-oadp-1.6"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/filebrowser-regenerate-package-lock-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/filebrowser-regenerate-pnpm-lock-and-commit.sh \
  "
