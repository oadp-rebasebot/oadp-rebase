#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Return the exit status of the last command in the pipe that failed

# Mark downstream fork modules as private so Go fetches them directly
# (bypassing the module proxy for fresh branch resolution) and skips
# the checksum database (sum.golang.org returns 404 for these).
export GOPRIVATE='github.com/openshift/*,github.com/migtools/*'
# Use the public proxy for all other modules (with direct fallback).
# This avoids failures when upstream repos are moved/deleted (e.g.
# lyft/protoc-gen-validate) since the proxy still has cached copies.
export GOPROXY='https://proxy.golang.org,direct'
# Skip checksum-database lookups for modules with known sumdb mismatches
# (e.g. envoyproxy republished tags with different content).
# openshift/* and migtools/* are already covered by GOPRIVATE above.
export GONOSUMDB='github.com/envoyproxy/*'

# Use whatever Go binary is installed — do not auto-download a newer
# toolchain and do not update the go or toolchain directives in go.mod.
# The actual Go version (and its CVE fixes) is controlled by the builder
# image, not by go.mod.
export GOTOOLCHAIN=local

stage_and_commit(){
    # If commiter email and name is passed as environment variable then use it.
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    # Hermeto validates vendor/ with `git add --intent-to-add --force`
    # (bypassing .gitignore) then diffs against committed state.
    # Without --force here, files matching .gitignore patterns (e.g.
    # go.work, go.work.sum) would be silently excluded, causing Hermeto
    # to detect them as missing and fail vendor consistency checks.
    # Must run BEFORE porcelain check since git status --porcelain
    # does not show ignored files.
    # See: https://github.com/hermetoproject/hermeto/blob/bb68e974/hermeto/core/package_managers/gomod/main.py#L1393
    if [ -d vendor ]; then
        git add --force vendor/
        if [[ -n $(git diff --cached --name-only -- vendor/) ]]; then
            echo "=== git add --force vendor/ staged previously ignored files ==="
            git diff --cached --name-only -- vendor/
        fi
    fi
    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Updating go modules"
    fi
}

process_go_mod_updates() {
    echo "Performing go modules update"

    find . -name 'go.mod' -print0 | while IFS= read -r -d '' go_mod_file; do
        local module_base_path
        module_base_path=$(dirname "$go_mod_file")

        # Reset go.mod and go.sum to make sure they are the same as in the source
        # for filename in "go.mod" "go.sum"; do
        #     local full_path="$module_base_path/$filename"
        #     if [[ ! -f "$full_path" ]]; then
        #         continue
        #     fi
        #     if ! git checkout "source/$REBASEBOT_SOURCE" -- "$full_path"; then
        #         echo "go module at $module_base_path is downstream only, skip its resetting"
        #         break
        #     fi
        # done

        pushd "$module_base_path"

        # Normalize go.mod: strip patch version from the go directive
        # (e.g. "go 1.25.7" -> "go 1.25") and remove the toolchain
        # directive entirely.  Combined with GOTOOLCHAIN=local this
        # prevents go mod tidy from re-adding them.
        echo "=== Normalizing go.mod in $module_base_path ==="
        sed -i -E 's/^go ([0-9]+\.[0-9]+)\.[0-9]+$/go \1/' go.mod
        sed -i '/^toolchain /d' go.mod

        # Remove go.sum before tidy to avoid "checksum mismatch" errors
        # caused by upstream modules being re-published with different
        # content (e.g. envoyproxy/go-control-plane). go mod tidy will
        # regenerate go.sum with correct hashes.
        rm -f go.sum

        echo "=== Running 'go mod tidy' in $module_base_path ==="
        if ! go mod tidy; then
            echo "Unable to run 'go mod tidy' in $module_base_path" >&2
            exit 1
        fi

        if [ -d "vendor" ]; then
            echo "=== vendor directory exists for $module_base_path"
            echo "=== go mod vendor output for $module_base_path"
            if ! go mod vendor; then
                echo "Unable to run 'go mod vendor' in $module_base_path" >&2
                exit 1
            fi
        else
            echo "=== No vendor directory found — skipping 'go mod vendor' ==="
        fi

        if [ "${GO_VET_SKIP:-}" = "1" ]; then
            echo "=== Skipping 'go vet' in $module_base_path (GO_VET_SKIP=1) ==="
        else
            echo "=== Running 'go vet' in $module_base_path ==="
            # go vet exits non-zero when there are no packages to vet (e.g. doc-only modules).
            # Detect that case and skip gracefully.
            vet_output=$(go vet ${GO_VET_TAGS:+-tags "$GO_VET_TAGS"} ./... 2>&1) || {
                if echo "$vet_output" | grep -q "no packages to vet\|matched no packages"; then
                    echo "=== No Go packages to vet in $module_base_path — skipping ==="
                else
                    echo "$vet_output" >&2
                    echo "Unable to run 'go vet' in $module_base_path" >&2
                    exit 1
                fi
            }
        fi

        popd
    done

    stage_and_commit
}

# Check if the source branch environment variable is set
if [[ -z "$REBASEBOT_SOURCE" ]]; then
    echo "The environment variable REBASEBOT_SOURCE is not set." >&2
    exit 1
fi

process_go_mod_updates
