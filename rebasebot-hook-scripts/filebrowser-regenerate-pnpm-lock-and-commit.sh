#!/bin/bash

set -e
set -o pipefail

# Regenerate frontend/pnpm-lock.yaml after rebase.
#
# After a rebase, upstream changes to package.json or transitive
# dependency shifts can leave pnpm-lock.yaml inconsistent. Both the
# downstream Containerfile and upstream GitHub Actions CI run
# `pnpm install --frozen-lockfile`, which rejects a stale lockfile.
#
# Prerequisites: node/npm must be available in the environment.

PNPM_VERSION="10.29.2"

stage_and_commit(){
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -- frontend/pnpm-lock.yaml
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Regenerate frontend/pnpm-lock.yaml"
    fi
}

regenerate_pnpm_lock() {
    if [ ! -f "frontend/pnpm-lock.yaml" ]; then
        echo "=== frontend/pnpm-lock.yaml not found — skipping pnpm lock regeneration ==="
        return 0
    fi

    echo "=== Regenerating frontend/pnpm-lock.yaml ==="

    if ! command -v pnpm &>/dev/null; then
        echo "pnpm not found, installing pnpm@${PNPM_VERSION} globally..."
        npm install -g "pnpm@${PNPM_VERSION}"
    fi

    pushd frontend

    pnpm install --no-frozen-lockfile

    popd

    stage_and_commit
}

regenerate_pnpm_lock
