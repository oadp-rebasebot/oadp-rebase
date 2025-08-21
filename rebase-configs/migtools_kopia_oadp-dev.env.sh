# For the oadp-dev, use the newest branch in a format e.g. v0.21.1-velero-patch

SOURCE_UPSTREAM_BRANCH=$(curl --silent --header "X-GitHub-Api-Version:2022-11-28" \
  "https://api.github.com/repos/project-velero/kopia/branches" \
  | grep -E '"name": "v[0-9]+\.[0-9]+\.[0-9]*-velero-patch"' \
  | awk -F'"' '{print $4}' \
  | sort -V \
  | tail -n 1)
  
SOURCE_UPSTREAM_REPO="https://github.com/project-velero/kopia:$SOURCE_UPSTREAM_BRANCH"
DESTINATION_DOWNSTREAM_REPO="migtools/kopia:oadp-dev"
REBASE_REPO="oadp-rebasebot/kopia:rebase-bot-oadp-dev"

