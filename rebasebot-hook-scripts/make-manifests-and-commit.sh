#!/bin/bash
set -euo pipefail

stage_and_commit(){
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q \
          -m "UPSTREAM: <drop>: make manifests update"
    fi
}

# Regenerate CRD manifests from Go types.
# Needed for repos whose CRDs embed Velero API types (e.g. BackupSpec,
# RestoreSpec) — a Velero dependency bump changes the generated CRD
# schema even when no local types changed.
make manifests

stage_and_commit
