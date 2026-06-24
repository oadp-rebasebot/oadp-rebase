#!/bin/sh
#
# Post /ok-to-test on open rebasebot PRs that don't have the label yet.
#
# Usage:
#   ./tools/ok-to-test.sh                          # dry-run, all branches
#   ./tools/ok-to-test.sh -b oadp-1.6              # dry-run, only oadp-1.6
#   ./tools/ok-to-test.sh -b oadp-1.6 --apply      # post on oadp-1.6 PRs
#
# Requires: gh CLI authenticated as an openshift org member.

set -eu

APPLY=false
BRANCH=""
while [ $# -gt 0 ]; do
    case "$1" in
        --apply) APPLY=true ;;
        -b|--branch)
            if [ $# -lt 2 ]; then echo "Error: $1 requires a branch name" >&2; exit 1; fi
            BRANCH="$2"; shift ;;
        -h|--help)
            echo "Usage: $0 [-b BRANCH] [--apply]"
            echo "  -b, --branch   Only target PRs for this branch (e.g. oadp-1.6)"
            echo "  --apply        Post /ok-to-test (default is dry-run)"
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
    shift
done

prs=$(gh search prs \
    --author "app/oadp-rebasebot-app" \
    --state open \
    --limit 50 \
    --json url,title,labels,repository)

needs_approval=$(echo "$prs" | jq -r \
    --arg branch "$BRANCH" '
    .[] |
    select([.labels[].name] | index("ok-to-test") | not) |
    select($branch == "" or (.title | endswith("into " + $branch))) |
    .url
')

if [ -z "$needs_approval" ]; then
    echo "All open rebasebot PRs already have ok-to-test."
    exit 0
fi

count=$(echo "$needs_approval" | wc -l | tr -d ' ')
echo "Found $count PR(s) missing ok-to-test:"
echo ""
echo "$needs_approval"
echo ""

if [ "$APPLY" = "false" ]; then
    echo "Run with --apply to post /ok-to-test on these PRs."
    exit 0
fi

for url in $needs_approval; do
    echo "Posting /ok-to-test on $url"
    gh pr comment "$url" --body "/ok-to-test"
done

echo "Done."
