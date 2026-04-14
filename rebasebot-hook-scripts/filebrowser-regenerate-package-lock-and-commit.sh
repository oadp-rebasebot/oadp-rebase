#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Return the exit status of the last command in the pipe that failed

# Regenerate frontend/package-lock.json for Konflux hermetic builds.
#
# Problem: Upstream filebrowser uses pnpm with pnpm-lock.yaml, but
# downstream Konflux (Cachi2) uses npm with package-lock.json.
# After a rebase, upstream changes to package.json can make the
# existing package-lock.json stale, causing npm clean-install to fail
# during the Konflux build.
#
# What this script does:
#   1. Checks if frontend/package-lock.json exists in the repo.
#      If it doesn't, the downstream branch doesn't need npm support
#      and the script exits cleanly.
#   2. Runs `npm install --package-lock-only` inside frontend/ to
#      regenerate package-lock.json from the current package.json.
#   3. Commits if anything changed.
#
# Prerequisites: npm must be available in the environment.

stage_and_commit(){
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -- frontend/package-lock.json
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Regenerate frontend/package-lock.json"
    fi
}

regenerate_package_lock() {
    if [ ! -f "frontend/package-lock.json" ]; then
        echo "=== frontend/package-lock.json not found — skipping npm lock regeneration ==="
        return 0
    fi

    echo "=== Regenerating frontend/package-lock.json ==="

    if ! command -v npm &>/dev/null; then
        echo "ERROR: npm is not available. Cannot regenerate package-lock.json." >&2
        exit 1
    fi

    pushd frontend

    # Regenerate package-lock.json without actually installing node_modules.
    # This updates the lock file to match the current package.json.
    npm install --package-lock-only --ignore-scripts --no-audit

    popd

    stage_and_commit
}

regenerate_package_lock
