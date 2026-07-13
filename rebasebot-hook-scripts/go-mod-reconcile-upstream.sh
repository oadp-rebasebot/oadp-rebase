#!/bin/bash
set -euo pipefail

# After cherry-picking with --conflict-policy warn (-Xtheirs), go.mod
# may have stale downstream versions pinned by <drop> commits. This
# hook compares each require against the upstream source go.mod and
# bumps any that were downgraded.  Replace directives are left alone
# — the existing go-replace hooks handle those.

if [[ -z "${REBASEBOT_SOURCE:-}" ]]; then
    echo "REBASEBOT_SOURCE is not set." >&2
    exit 1
fi

stage_and_commit() {
    if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q \
            -m "UPSTREAM: <drop>: reconcile go.mod versions with upstream"
    fi
}

# Build a temporary semver-compare binary that uses golang.org/x/mod/semver
# for correct Go module version ordering (sort -V gets pre-release wrong).
# Canonical tested implementation: tools/semver-compare/
SEMVER_COMPARE_DIR=$(mktemp -d)
trap 'rm -rf "$SEMVER_COMPARE_DIR"' EXIT
(
    cd "$SEMVER_COMPARE_DIR"
    go mod init semver-compare >/dev/null 2>&1
    go get golang.org/x/mod/semver >/dev/null 2>&1
    cat > main.go <<'GOEOF'
package main

import (
	"fmt"
	"os"

	"golang.org/x/mod/semver"
)

func main() {
	a, b := os.Args[1], os.Args[2]
	if !semver.IsValid(a) || !semver.IsValid(b) {
		// Fall back to lexicographic for non-semver (e.g., go directive "1.22")
		if a >= b { fmt.Println(a) } else { fmt.Println(b) }
		return
	}
	if semver.Compare(a, b) >= 0 { fmt.Println(a) } else { fmt.Println(b) }
}
GOEOF
    go build -o semver-compare . >/dev/null 2>&1
)
SEMVER_COMPARE="$SEMVER_COMPARE_DIR/semver-compare"

version_max() {
    "$SEMVER_COMPARE" "$1" "$2"
}

reconcile_gomod() {
    local gomod_path="$1"
    local module_dir
    module_dir=$(dirname "$gomod_path")

    local upstream_gomod
    upstream_gomod=$(git show "source/${REBASEBOT_SOURCE}:${gomod_path}" 2>/dev/null) || {
        echo "=== $gomod_path: not in upstream — skipping ==="
        return
    }

    echo "=== Reconciling $gomod_path with upstream ==="

    local upstream_tmp
    upstream_tmp=$(mktemp)
    echo "$upstream_gomod" > "$upstream_tmp"

    pushd "$module_dir" > /dev/null

    # Build map of upstream requires (both direct and indirect)
    local -A upstream_reqs=()
    while IFS=$'\t' read -r mod ver; do
        [[ -n "$mod" && -n "$ver" ]] && upstream_reqs["$mod"]="$ver"
    done < <(go mod edit -json "$upstream_tmp" | \
        jq -r '.Require[]? | "\(.Path)\t\(.Version)"')

    # Build map of current requires
    local -A current_reqs=()
    while IFS=$'\t' read -r mod ver; do
        [[ -n "$mod" && -n "$ver" ]] && current_reqs["$mod"]="$ver"
    done < <(go mod edit -json | \
        jq -r '.Require[]? | "\(.Path)\t\(.Version)"')

    local bumped=0

    for mod in "${!upstream_reqs[@]}"; do
        local up_ver="${upstream_reqs[$mod]}"

        if [[ -v "current_reqs[$mod]" ]]; then
            local cur_ver="${current_reqs[$mod]}"
            local max_ver
            max_ver=$(version_max "$cur_ver" "$up_ver")
            if [[ "$max_ver" != "$cur_ver" ]]; then
                echo "  bump $mod: $cur_ver -> $max_ver"
                go mod edit -require="${mod}@${max_ver}"
                bumped=$((bumped + 1))
            fi
        fi
    done

    # Bump the go directive if upstream is higher
    local up_go cur_go
    up_go=$(go mod edit -json "$upstream_tmp" | jq -r '.Go // empty')
    cur_go=$(go mod edit -json | jq -r '.Go // empty')
    if [[ -n "$up_go" && -n "$cur_go" ]]; then
        local max_go
        max_go=$(version_max "$cur_go" "$up_go")
        if [[ "$max_go" != "$cur_go" ]]; then
            echo "  bump go directive: $cur_go -> $max_go"
            go mod edit -go="$max_go"
            bumped=$((bumped + 1))
        fi
    fi

    popd > /dev/null
    rm -f "$upstream_tmp"

    if [[ "$bumped" -eq 0 ]]; then
        echo "  no changes needed"
    else
        echo "  $bumped updates applied"
    fi
}

while IFS= read -r -d '' gomod; do
    gomod="${gomod#./}"
    reconcile_gomod "$gomod"
done < <(find . -name 'go.mod' -not -path './vendor/*' -print0)

stage_and_commit
