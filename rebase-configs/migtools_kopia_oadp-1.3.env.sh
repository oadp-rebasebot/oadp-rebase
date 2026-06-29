# ===============================================================
# Kopia rebase configuration for OADP 1.3
# Upstream tag sourced from versions/oadp-1.3.env
# ===============================================================

UPSTREAM_KOPIA_REPO="project-velero/kopia"
UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO="$KOPIA_UPSTREAM_TAG"
DESTINATION_DOWNSTREAM_BRANCH="oadp-1.3"

SOURCE_UPSTREAM_REPO="https://github.com/$UPSTREAM_KOPIA_REPO:$UPSTREAM_KOPIA_TAG_BRANCH_FOR_VELERO"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:$DESTINATION_DOWNSTREAM_BRANCH"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-$DESTINATION_DOWNSTREAM_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/normalize-dockerfiles-and-commit.sh \
  "
