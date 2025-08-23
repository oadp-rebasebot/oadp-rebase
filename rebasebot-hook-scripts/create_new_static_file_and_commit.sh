#!/bin/bash
set -euo pipefail

# Similar to: https://github.com/openshift-eng/rebasebot/blob/846d846969accb3c2eababc8b23cc73ed3d484ca/rebasebot/builtin-hooks/update_go_modules.sh#L6
stage_and_commit(){
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Use new static text"
    fi
}

echo "new file" > ./rebasebot-text.txt

stage_and_commit

echo "Static file ./rebasebot-new-text.txt updated to the latest."


