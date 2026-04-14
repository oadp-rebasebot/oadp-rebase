#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Return the exit status of the last command in the pipe that failed

# Normalize Go builder image versions in Dockerfiles / Containerfiles.
#
# Problem: Dockerfiles that pin a Go patch version (e.g. golang:1.25.7)
# miss future CVE fixes because the tag never moves.  Using just the
# minor version (golang:1.25) always resolves to the latest patch.
#
# What this script does:
#   1. Finds all Dockerfile* and Containerfile* in the repository.
#   2. In FROM lines that reference the upstream "golang:" image,
#      strips the patch version: golang:1.25.7-bookworm -> golang:1.25-bookworm
#   3. Leaves non-golang images untouched (konveyor/builder, brew.registry,
#      registry.ci.openshift.org, ubi, distroless, etc.).
#   4. Commits if anything changed.
#
# This is safe because:
#   - "golang:1.25" is a floating tag that always points to the latest
#     1.25.x release, which includes security fixes.
#   - Downstream builds (konflux.Dockerfile, Dockerfile.ubi) already use
#     Red Hat builder images that don't pin patch versions.

stage_and_commit(){
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Normalize Go version in Dockerfiles"
    fi
}

normalize_dockerfiles() {
    echo "Normalizing Go builder versions in Dockerfiles"

    local changed=0

    # Find all Dockerfile* and Containerfile* (any depth)
    while IFS= read -r -d '' dockerfile; do
        # Only process files that have a golang: FROM line with a patch version.
        # Match patterns like:
        #   golang:1.25.7
        #   golang:1.25.7-bookworm
        #   golang:1.23.6-bullseye
        #   golang:1.25.0
        # But NOT:
        #   golang:1.25
        #   golang:1.25-bookworm  (already correct)
        #   brew.registry.redhat.io/...  (not "golang:" prefix)
        #   registry.ci.openshift.org/... (not "golang:" prefix)
        #   quay.io/konveyor/builder:... (not "golang:" prefix)
        if grep -qE 'FROM\s+.*\bgolang:[0-9]+\.[0-9]+\.[0-9]+' "$dockerfile"; then
            echo "=== Normalizing $dockerfile ==="

            # Strip patch version from golang: image references.
            # golang:1.25.7-bookworm  ->  golang:1.25-bookworm
            # golang:1.25.8           ->  golang:1.25
            # golang:1.23.6-bullseye  ->  golang:1.23-bullseye
            # golang:1.25.0           ->  golang:1.25
            sed -i -E 's/(golang:[0-9]+\.[0-9]+)\.[0-9]+/\1/g' "$dockerfile"

            changed=1
        else
            echo "=== Skipping $dockerfile (no golang patch version found) ==="
        fi
    done < <(find . \( -name 'Dockerfile*' -o -name 'Containerfile*' \) -not -path '*/vendor/*' -print0)

    if [ "$changed" -eq 0 ]; then
        echo "=== No Dockerfiles needed normalization ==="
    fi

    stage_and_commit
}

normalize_dockerfiles
