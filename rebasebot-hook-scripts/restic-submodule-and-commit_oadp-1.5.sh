#!/bin/bash
set -euo pipefail

SUBMODULE_BRANCH="oadp-1.5"
SUBMODULE_URL="https://github.com/openshift/restic"
SUBMODULE_PATH="restic"

# Similar to: https://github.com/openshift-eng/rebasebot/blob/846d846969accb3c2eababc8b23cc73ed3d484ca/rebasebot/builtin-hooks/update_go_modules.sh#L6
stage_and_commit(){
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        # short hash of the submodule
        local submodule_short_hash
        submodule_short_hash=$(git -C "$SUBMODULE_PATH" rev-parse --short HEAD)

        git add -A
        git commit "${author_flag[@]}" -q \
          -m "UPSTREAM: <drop>: update restic @ ${submodule_short_hash} (branch $SUBMODULE_BRANCH)"
    fi
}

# If submodule missing, add it
if ! git config --file .gitmodules --get "submodule.$SUBMODULE_PATH.url" >/dev/null 2>&1; then
    echo "Adding submodule $SUBMODULE_PATH..."
    git submodule add -b "$SUBMODULE_BRANCH" "$SUBMODULE_URL" "$SUBMODULE_PATH"
else
    git config -f .gitmodules "submodule.$SUBMODULE_PATH.branch" "$SUBMODULE_BRANCH"
fi

git submodule sync "$SUBMODULE_PATH"
git submodule update --init --remote --rebase "$SUBMODULE_PATH"

stage_and_commit

echo "Submodule $SUBMODULE_PATH updated to the latest $SUBMODULE_BRANCH."

