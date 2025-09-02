# ===============================================================
# Fetch the latest minor Velero release for a given major version.
# ===============================================================

UPSTREAM_VELERO_MAJOR_VERSION=v1.17
DESTINATION_DOWNSTREAM_VELERO_BRANCH=oadp-dev

# Include -rc.X versions, this is oadp-dev, so latest tagged Velero
LATEST_VELERO_TAG=$(
  curl -s -L "https://api.github.com/repos/vmware-tanzu/velero/tags?per_page=100" \
  | grep -Eo "\"name\": \"${UPSTREAM_VELERO_MAJOR_VERSION}\.[0-9]+(-rc\.[0-9]+)?\"" \
  | awk -F'"' '{print $4}' \
  | sort -V \
  | tail -n 1
)

SOURCE_UPSTREAM_REPO="https://github.com/vmware-tanzu/velero:$LATEST_VELERO_TAG"
DESTINATION_DOWNSTREAM_REPO="openshift/velero:$DESTINATION_DOWNSTREAM_VELERO_BRANCH"
REBASE_REPO="oadp-rebasebot/velero:rebase-bot-$DESTINATION_DOWNSTREAM_VELERO_BRANCH"

EXTRA_REBASEBOT_ARGS="--always-run-hooks"
HOOK_SCRIPTS="--post-rebase-hook git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts/go-replace_kopia_oadp-dev.sh git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts/go-mod-tidy-and-commit.sh git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:restic-submodule-and-commit_oadp-dev.sh"
