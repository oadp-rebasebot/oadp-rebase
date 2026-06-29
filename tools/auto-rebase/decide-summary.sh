#!/bin/sh
#
# Generates a markdown decision summary from rebase-status JSON files.
# Outputs to stdout — caller redirects to $GITHUB_STEP_SUMMARY or a file.
#
# Usage:
#   decide-summary.sh --targets '<json>' /tmp/status-oadp-1.6.json [...]

set -eu

TARGETS='[]'
FILES=""

while [ $# -gt 0 ]; do
    case "$1" in
        --targets) TARGETS="$2"; shift 2 ;;
        -h|--help)
            echo "Usage: decide-summary.sh --targets '<json>' <status-json-file> [...]"
            exit 0
            ;;
        -*) echo "Unknown option: $1" >&2; exit 1 ;;
        *)  FILES="${FILES:+$FILES }$1"; shift ;;
    esac
done

[ -z "$FILES" ] && { echo "Error: at least one status JSON file required" >&2; exit 1; }

echo "## Rebase Decision Summary"
echo ""

for json_file in $FILES; do
    [ -f "$json_file" ] || continue

    branch=$(jq -r '.[0].branch // "unknown"' < "$json_file")

    echo "### Branch: \`$branch\`"
    echo ""

    selected=$(echo "$TARGETS" | jq -r ".[] | select(.branch == \"$branch\") | \"| \(.target) | \(.reason) |\"")
    if [ -n "$selected" ]; then
        echo "**Selected for rebase:**"
        echo ""
        echo "| Target | Reason |"
        echo "|--------|--------|"
        echo "$selected"
    else
        echo "No targets selected for rebase."
    fi

    skipped=$(jq -r '
        [ .[] | select(.skip == true) | {repo: (.repo | split("/") | .[1]), reason: "skip=true"} ] +
        [ .[] | select(.skip != true) | select(.checks.config.status != "ok") | {repo: (.repo | split("/") | .[1]), reason: "no rebase config"} ] +
        [ .[] | select(.skip != true) | select(.checks.config.status == "ok") | select(.checks.open_pr.status == "ok") | {repo: (.repo | split("/") | .[1]), reason: "open PR exists"} ] +
        [ .[] | select(.skip != true) | select(.checks.config.status == "ok") | select(.checks.open_pr.status != "ok") | select(.wave >= 2) | select(.checks.dep_sync.status != "fail") | {repo: (.repo | split("/") | .[1]), reason: "deps in sync"} ]
        | sort_by(.repo) | .[] | "| \(.repo) | \(.reason) |"
    ' < "$json_file")

    if [ -n "$skipped" ]; then
        echo ""
        echo "**Skipped:**"
        echo ""
        echo "| Repo | Reason |"
        echo "|------|--------|"
        echo "$skipped"
    fi
    echo ""
done
