#!/bin/bash
set -euo pipefail

# After cherry-picking with --conflict-policy warn (-Xtheirs), go.mod
# may have stale downstream versions pinned by <drop> commits. This
# hook compares each require against the upstream source go.mod and
# bumps any that were downgraded.  Replace directives are left alone
# — the existing go-replace hooks handle those.
#
# Results are visible in the rebase PR as the commit diff for
# "UPSTREAM: <drop>: reconcile go.mod versions with upstream".

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
# Non-semver inputs (e.g., Go directive "1.22") are normalized to valid semver
# before comparison so that 1.9 < 1.10 is handled correctly.
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
	"strings"

	"golang.org/x/mod/semver"
)

func normalize(s string) string {
	v := s
	if !strings.HasPrefix(v, "v") { v = "v" + v }
	parts := strings.SplitN(v[1:], ".", 3)
	for len(parts) < 3 { parts = append(parts, "0") }
	return "v" + strings.Join(parts, ".")
}

func main() {
	a, b := os.Args[1], os.Args[2]
	if semver.IsValid(a) && semver.IsValid(b) {
		if semver.Compare(a, b) >= 0 { fmt.Println(a) } else { fmt.Println(b) }
		return
	}
	na, nb := normalize(a), normalize(b)
	if semver.IsValid(na) && semver.IsValid(nb) {
		if semver.Compare(na, nb) >= 0 { fmt.Println(a) } else { fmt.Println(b) }
		return
	}
	if a >= b { fmt.Println(a) } else { fmt.Println(b) }
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
    local upstream_json
    upstream_json=$(go mod edit -json "$upstream_tmp") || {
        echo "  ERROR: failed to parse upstream go.mod" >&2
        popd > /dev/null; rm -f "$upstream_tmp"; return 1
    }

    local -A upstream_reqs=()
    while IFS=$'\t' read -r mod ver; do
        [[ -n "$mod" && -n "$ver" ]] && upstream_reqs["$mod"]="$ver"
    done < <(echo "$upstream_json" | jq -r '.Require[]? | "\(.Path)\t\(.Version)"')

    # go mod edit (both -json reads and -require/-go writes) rejects go.mod
    # files containing non-semver replace directives such as branch names
    # (e.g., "oadp-1.4") written by the go-replace hooks. When that happens,
    # temporarily strip all replace directives (block and single-line) so
    # every go mod edit call in this function succeeds, then restore them.
    local has_nonsemver_replace=false
    local gomod_replace_backup=""
    local current_json
    current_json=$(go mod edit -json 2>/dev/null) || true
    if [[ -z "$current_json" ]]; then
        has_nonsemver_replace=true
        gomod_replace_backup=$(mktemp)
        cp go.mod "$gomod_replace_backup"
        sed -i '/^replace (/,/^)/d; /^replace /d' go.mod
        current_json=$(go mod edit -json) || {
            echo "  ERROR: failed to parse current go.mod (even after stripping replaces)" >&2
            cp "$gomod_replace_backup" go.mod
            popd > /dev/null; rm -f "$upstream_tmp" "$gomod_replace_backup"; return 1
        }
    fi

    local -A current_reqs=()
    while IFS=$'\t' read -r mod ver; do
        [[ -n "$mod" && -n "$ver" ]] && current_reqs["$mod"]="$ver"
    done < <(echo "$current_json" | jq -r '.Require[]? | "\(.Path)\t\(.Version)"')

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
    up_go=$(echo "$upstream_json" | jq -r '.Go // empty')
    cur_go=$(echo "$current_json" | jq -r '.Go // empty')
    if [[ -n "$up_go" && -n "$cur_go" ]]; then
        local max_go
        max_go=$(version_max "$cur_go" "$up_go")
        if [[ "$max_go" != "$cur_go" ]]; then
            echo "  bump go directive: $cur_go -> $max_go"
            go mod edit -go="$max_go"
            bumped=$((bumped + 1))
        fi
    fi

    # Restore replace directives that were stripped for editing
    if [[ "$has_nonsemver_replace" == "true" && -n "$gomod_replace_backup" ]]; then
        sed -n '/^replace (/,/^)/p' "$gomod_replace_backup" >> go.mod
        grep '^replace [^(]' "$gomod_replace_backup" >> go.mod || true
        rm -f "$gomod_replace_backup"
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
done < <(find . -name 'go.mod' -not -path '*/vendor/*' -print0)

stage_and_commit
