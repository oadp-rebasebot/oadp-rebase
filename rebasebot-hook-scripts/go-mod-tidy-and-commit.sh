#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Return the exit status of the last command in the pipe that failed

# Bypass the Go module proxy to resolve branch-based replace directives
# to the latest commit on the branch, avoiding stale cached versions.
export GOPROXY=direct
# Skip checksum database verification for downstream fork modules that
# are not published to the Go ecosystem (sum.golang.org returns 404).
export GONOSUMDB='github.com/openshift/*,github.com/migtools/*'
export GONOSUMCHECK='github.com/openshift/*,github.com/migtools/*'

stage_and_commit(){
    # If commiter email and name is passed as environment variable then use it.
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        # Hermeto validates vendor/ with `git add --intent-to-add --force`
        # (bypassing .gitignore) then diffs against committed state.
        # Without --force here, files matching .gitignore patterns (*.so,
        # *.exe, etc.) would be silently excluded, causing Hermeto to
        # detect them as missing and fail vendor consistency checks.
        # See: https://github.com/hermetoproject/hermeto/blob/bb68e974/hermeto/core/package_managers/gomod/main.py#L1393
        git add --force vendor/ 2>/dev/null || true
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

        echo "=== Running 'go vet' in $module_base_path ==="
        if ! go vet ./...; then
            echo "Unable to run 'go vet' in $module_base_path" >&2
            exit 1
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
