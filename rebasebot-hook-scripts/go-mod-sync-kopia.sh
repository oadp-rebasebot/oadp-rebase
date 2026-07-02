#!/bin/bash
set -euo pipefail

# Reset go.mod to the source upstream (migtools/kopia) so that
# downstream carry commits that pin stale dependency versions
# are overwritten with the current kopia versions.
# Run BEFORE go-mod-tidy-and-commit.sh — tidy will add any
# extra deps oadp-vmdp needs on top of the kopia base.

if [[ -z "${REBASEBOT_SOURCE:-}" ]]; then
    echo "REBASEBOT_SOURCE is not set." >&2
    exit 1
fi

echo "=== Syncing go.mod from source/${REBASEBOT_SOURCE} ==="
git checkout "source/${REBASEBOT_SOURCE}" -- go.mod

stage_and_commit(){
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q \
          -m "UPSTREAM: <drop>: sync go.mod from kopia source"
    fi
}

stage_and_commit
