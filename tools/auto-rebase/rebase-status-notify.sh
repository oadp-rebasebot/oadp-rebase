#!/bin/sh
#
# Builds a Slack Block Kit payload from rebase-status --json output.
# Shows pending rebase PRs and out-of-sync dependencies.
#
# Usage:
#   rebase-status --json oadp-1.6 > /tmp/status.json
#   rebase-status-notify.sh /tmp/status-oadp-1.6.json [/tmp/status-oadp-1.5.json ...]
#   rebase-status-notify.sh --branch oadp-1.6 < status.json
#
# Options:
#   --branch NAME   Label for the branch when reading from a single file/stdin
#   --wiki-url URL  Link to the wiki status page
#   --gh-token TOK  GitHub token for checking empty PRs (changed_files == 0)
#   -h, --help      Show this help

set -eu

BRANCH=""
WIKI_URL="https://github.com/oadp-rebasebot/oadp-rebase/wiki"
GH_TOKEN=""
FILES=""

while [ $# -gt 0 ]; do
    case "$1" in
        --branch)   BRANCH="$2"; shift 2 ;;
        --wiki-url) WIKI_URL="$2"; shift 2 ;;
        --gh-token) GH_TOKEN="$2"; shift 2 ;;
        -h|--help)
            sed -n '2,/^$/s/^# //p' "$0"
            exit 0
            ;;
        -*) echo "Unknown option: $1" >&2; exit 1 ;;
        *)  FILES="${FILES:+$FILES }$1"; shift ;;
    esac
done

# Read input: files or stdin
if [ -n "$FILES" ]; then
    input=""
    for f in $FILES; do
        # Extract .repos array from object format, fall back to raw array
        content=$(jq 'if type == "object" then .repos else . end' "$f")
        if [ -z "$input" ]; then
            input="$content"
        else
            input=$(echo "$input" "$content" | jq -s 'add')
        fi
    done
else
    # Extract .repos array from object format, fall back to raw array
    input=$(cat | jq 'if type == "object" then .repos else . end')
fi

# Detect branch from JSON if not provided
if [ -z "$BRANCH" ]; then
    BRANCH=$(echo "$input" | jq -r '[.[].branch] | unique | join(", ")' 2>/dev/null || echo "unknown")
fi

# Check for empty PRs (changed_files == 0) if we have a token
# Build a JSON object mapping "org/repo/#num" -> changed_files
pr_metadata="{}"
if [ -n "$GH_TOKEN" ]; then
    tmpfile=$(mktemp)
    echo "{" > "$tmpfile"
    echo "$input" | jq -r '
        .[] | select(.checks.open_pr.status == "ok") |
        .repo + " " + .checks.open_pr.summary
    ' 2>/dev/null | while IFS=' ' read -r repo pr_num; do
        [ -z "$repo" ] && continue
        num=$(echo "$pr_num" | tr -d '#')
        changed=$(curl -sf -H "Authorization: token $GH_TOKEN" \
            "https://api.github.com/repos/$repo/pulls/$num" 2>/dev/null | \
            jq '.changed_files // -1' 2>/dev/null || echo "-1")
        echo "\"${repo}/${pr_num}\":${changed}," >> "$tmpfile"
    done
    # Remove trailing comma and close
    sed -i.bak '$ s/,$//' "$tmpfile" 2>/dev/null || sed -i '' '$ s/,$//' "$tmpfile"
    echo "}" >> "$tmpfile"
    pr_metadata=$(cat "$tmpfile")
    rm -f "$tmpfile" "$tmpfile.bak"
fi

# Build the Slack payload
echo "$input" | jq --arg wiki_url "$WIKI_URL" \
    --arg branch_label "$BRANCH" \
    --argjson pr_meta "$pr_metadata" \
'
# Split repos by branch
(group_by(.branch) | sort_by(.[0].branch)) as $groups |

# Build sections per branch
[$groups[] |
    .[0].branch as $branch |

    # Pending PRs — construct URL from repo + summary, exclude empty PRs
    ([.[] | select(.checks.open_pr.status == "ok") |
        (.checks.open_pr.summary | ltrimstr("#")) as $num |
        ("https://github.com/" + .repo + "/pull/" + $num) as $pr_url |
        ($pr_meta[.repo + "/" + .checks.open_pr.summary] // -1) as $changed |
        select($changed != 0) |
        {
            repo: .repo,
            wave: .wave,
            pr_num: .checks.open_pr.summary,
            pr_url: $pr_url
        }
    ]) as $prs |

    # Upstream or dependencies out of sync without a PR (pending work)
    [.[] | select((.checks.upstream_sync.status == "fail" or .checks.dep_sync.status == "fail") and .checks.open_pr.status != "ok") |
        {
            repo: .repo,
            wave: .wave,
            rebase_reason: (if .checks.upstream_sync.status == "fail" then "upstream changed" else "dependencies changed" end)
        }
    ] as $rebase_pending |

    # Skip branch if nothing actionable
    select(($prs | length) > 0 or ($rebase_pending | length) > 0) |

    {
        branch: $branch,
        prs: $prs,
        rebase_pending: $rebase_pending
    }
] as $sections |

# If nothing actionable across all branches, emit empty payload
if ($sections | length) == 0 then
    {blocks: [], empty: true}
else

[
    {type: "header", text: {type: "plain_text", text: "OADP Rebase Status"}},

    ($sections[] |
        (
            "*" + .branch + "*\n" +

            (if (.prs | length) > 0 then
                ":arrows_counterclockwise: *" + (.prs | length | tostring) + " pending PR" +
                (if (.prs | length) > 1 then "s" else "" end) + "*\n" +
                (.prs | sort_by(.wave) | map(
                    ":large_orange_circle:" +
                    " `" + (.repo | split("/") | .[1]) + "` " +
                    "<" + .pr_url + "|" + .pr_num + ">" +
                    " (W" + (.wave | tostring) + ")"
                ) | join("\n"))
            else "" end) +

            (if (.rebase_pending | length) > 0 then
                (if (.prs | length) > 0 then "\n" else "" end) +
                ":warning: *" + (.rebase_pending | length | tostring) +
                " rebase target" + (if (.rebase_pending | length) > 1 then "s" else "" end) +
                " pending* (no PR)\n" +
                (.rebase_pending | sort_by(.wave) | map(
                    ":small_orange_diamond: `" + (.repo | split("/") | .[1]) +
                    "` " + .rebase_reason + " (W" + (.wave | tostring) + ")"
                ) | join("\n"))
            else "" end)
        ) | {type: "section", text: {type: "mrkdwn", text: .}}
    ),

    {type: "context", elements: [{type: "mrkdwn",
        text: ("<" + $wiki_url + "|Wiki> | <https://oadp-rebasebot.github.io/oadp-rebase/rebase-dashboard/|Dashboard> | <https://oadp-rebasebot.github.io/oadp-rebase/rebase-dag/|Dependency Graph> | " + (now | strftime("%Y-%m-%d %H:%M UTC")))
    }]}
] | {blocks: .}

end
'
