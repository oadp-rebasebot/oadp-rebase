#!/bin/bash
set -euo pipefail

DOWNSTREAM_VELERO_REPO="openshift/velero"
DOWNSTREAM_VELERO_BRANCH="oadp-dev"
TARGET_DIR="config/crd/bases"


# Similar to: https://github.com/openshift-eng/rebasebot/blob/846d846969accb3c2eababc8b23cc73ed3d484ca/rebasebot/builtin-hooks/update_go_modules.sh#L6
stage_and_commit(){
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q \
          -m "UPSTREAM: <drop>: update Velero CRDs @ ${DOWNSTREAM_VELERO_BRANCH}"
    fi
}

# Download CRDs from openshift/velero
for folder in v1/bases v2alpha1/bases; do
  api_url="https://api.github.com/repos/$DOWNSTREAM_VELERO_REPO/contents/config/crd/$folder?ref=$VELERO_BRANCH"
  for file in $(curl -s "$api_url" | jq -r '.[].download_url'); do
    echo "Downloading: $file"
    echo "Saving to: $TARGET_DIR/$(basename "$file")"
    curl -sL "$file" -o "$TARGET_DIR/$(basename "$file")"
  done
done

stage_and_commit
