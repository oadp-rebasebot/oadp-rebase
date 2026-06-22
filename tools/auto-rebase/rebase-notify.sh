#!/bin/sh
#
# Builds a Slack Block Kit payload from auto-rebase result files.
# Outputs JSON to stdout — caller pipes to curl or saves to file.
#
# Usage:
#   rebase-notify.sh <results-dir> [--dry-run] [--run-url URL] [--no-targets]
#
# Result JSON format (one file per repo in results-dir):
#   {"target": "velero-oadp-dev", "branch": "oadp-dev", "reason": "deps-changed",
#    "exit_code": "0", "pr_url": "https://github.com/.../pull/123"}

set -eu

RESULTS_DIR=""
DRY_RUN=false
RUN_URL=""
NO_TARGETS=false

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)    DRY_RUN=true; shift ;;
        --run-url)    RUN_URL="$2"; shift 2 ;;
        --no-targets) NO_TARGETS=true; shift ;;
        -h|--help)
            cat <<'USAGE'
Builds a Slack Block Kit payload from auto-rebase result files.
Outputs JSON to stdout.

Usage:
  rebase-notify.sh <results-dir> [--dry-run] [--run-url URL] [--no-targets]

Options:
  --dry-run      Add [DRY RUN] label to header
  --run-url URL  Link to the workflow run
  --no-targets   Generate "nothing to do" message (ignores results-dir)
  -h, --help     Show this help
USAGE
            exit 0
            ;;
        -*)
            echo "Unknown option: $1" >&2; exit 1 ;;
        *)
            RESULTS_DIR="$1"; shift ;;
    esac
done

dry_label=""
if [ "$DRY_RUN" = "true" ]; then
    dry_label=" [DRY RUN]"
fi

context_text=""
if [ -n "$RUN_URL" ]; then
    context_text="<${RUN_URL}|Workflow run>"
fi

# No targets — short message
if [ "$NO_TARGETS" = "true" ]; then
    jq -n \
        --arg dry_label "$dry_label" \
        --arg context "$context_text" \
    '
    [
        {type: "header", text: {type: "plain_text", text: ("OADP Auto-Rebase" + $dry_label)}},
        {type: "section", text: {type: "mrkdwn", text: ":zzz: No repos need rebasing today."}}
    ] + (if $context != "" then [{type: "context", elements: [{type: "mrkdwn", text: $context}]}] else [] end)
    | {blocks: .}
    '
    exit 0
fi

[ -z "$RESULTS_DIR" ] && { echo "Error: results directory required" >&2; exit 1; }

# Collect all result files into a single JSON array
results='[]'
if [ -d "$RESULTS_DIR" ]; then
    for file in "$RESULTS_DIR"/*.json; do
        [ -f "$file" ] || continue
        results=$(echo "$results" | jq --slurpfile r "$file" '. + $r')
    done
fi

# Build the Slack body and stats from the collected results
jq -n \
    --arg dry_label "$dry_label" \
    --arg context "$context_text" \
    --argjson results "$results" \
'
def branch_order:
    if . == "oadp-dev" then 0
    elif . == "oadp-1.6" then 1
    elif . == "oadp-1.5" then 2
    else 3 end;

($results | group_by(.branch) | sort_by(.[0].branch | branch_order)) as $groups |

($results | map(select(.exit_code == "0")) | length) as $succeeded |
($results | map(select(.exit_code != "0")) | length) as $failed |
($results | length) as $total |

($groups | map(
    "*" + .[0].branch + "*\n" +
    (map(
        (if .exit_code == "0" then ":white_check_mark:" else ":x:" end) +
        " `" + .target + "`" +
        (if .reason != "" and .reason != null then " (" + .reason + ")" else "" end) +
        (if .pr_url != "" and .pr_url != null then " — <" + .pr_url + "|PR>" else "" end)
    ) | join("\n"))
) | join("\n\n")) as $body |

(
    (($succeeded | tostring) + "/" + ($total | tostring) + " succeeded") +
    (if $failed > 0 then ", " + ($failed | tostring) + " failed" else "" end)
) as $summary |

[
    {type: "header", text: {type: "plain_text", text: ("OADP Auto-Rebase" + $dry_label)}},
    {type: "section", text: {type: "mrkdwn", text: $body}}
] + (if $context != "" then
    [{type: "context", elements: [{type: "mrkdwn", text: ($summary + " | " + $context)}]}]
else
    [{type: "context", elements: [{type: "mrkdwn", text: $summary}]}]
end)
| {blocks: .}
'
