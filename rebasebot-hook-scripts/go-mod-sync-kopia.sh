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

# Preserve the destination's go directive when it is newer. Downstream carry
# commits may bump the language version (e.g. to use Go 1.26 features like
# new(expr)); resetting go.mod must not downgrade it, or `go vet` fails with
# "requires go1.26 or later (-lang was set to go1.25)".
preserve_go_directive() {
    local dest_go src_go
    dest_go=$(git show HEAD:go.mod | awk '/^go /{print $2; exit}')
    src_go=$(awk '/^go /{print $2; exit}' go.mod)
    if [ -n "$dest_go" ] && [ -n "$src_go" ] && [ "$dest_go" != "$src_go" ]; then
        if [ "$(printf '%s\n%s\n' "$src_go" "$dest_go" | sort -V | tail -1)" = "$dest_go" ]; then
            echo "=== Preserving destination go directive: go $dest_go (source: go $src_go) ==="
            sed -i.bak "s|^go .*|go $dest_go|" go.mod && rm -f go.mod.bak
        fi
    fi
}
preserve_go_directive

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
