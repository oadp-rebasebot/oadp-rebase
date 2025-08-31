# The SOURCE and DESTINATION repo and branch are the same
# It is a hack to ensure we don't really rebase, but we run the hooks
SOURCE_UPSTREAM_REPO="https://github.com/migtools/udistribution:main"
DESTINATION_DOWNSTREAM_REPO="migtools/udistribution:main"
REBASE_REPO="oadp-rebasebot/udistribution:rebase-bot-main"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS="--post-rebase-hook git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts/go-replace_docker-distribution_main.sh git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts/go-mod-tidy-and-commit.sh"
