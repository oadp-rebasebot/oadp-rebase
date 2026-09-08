#!/bin/sh
#
# Reads rebase-status --json from stdin and outputs repo-branch targets
# that are eligible for a rebasebot run.
#
# Usage:
#   rebase-status --json --hide-dependency-details oadp-dev | rebase-decision.sh [--reason] [--waves 1,2]
#
# Decision logic per repo:
#   1. Skip if skip == true
#   2. Skip if checks.config.status != "ok" (no config or NoRebase)
#   3. Wave 1: always eligible (no internal deps)
#   4. Wave 2+: eligible when their configured upstream or an internal
#      dependency has commits not yet in the downstream branch.
#
# Output: one target per line, sorted by wave, e.g. "velero-oadp-dev"
#   --reason: append tab-separated reason (wave1-always | upstream-changed | deps-changed)

set -eu

SHOW_REASON=false
WAVES=""

while [ $# -gt 0 ]; do
    case "$1" in
        --reason) SHOW_REASON=true; shift ;;
        --waves)
            [ $# -ge 2 ] || { echo "--waves requires a value" >&2; exit 1; }
            WAVES="$2"
            shift 2
            ;;
        -h|--help)
            cat <<'USAGE'
Reads rebase-status --json from stdin and outputs repo-branch targets
that are eligible for a rebasebot run.

Usage:
  rebase-status --json --hide-dependency-details oadp-dev | rebase-decision.sh [--reason] [--waves 1,2]

Options:
  --reason    Append tab-separated reason (wave1-always | upstream-changed | deps-changed)
  --waves     Comma-separated waves to include; empty includes all waves
  -h, --help  Show this help
USAGE
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [ -n "$WAVES" ]; then
    case "$WAVES" in
        *[!0-9,]*|,*|*,|*,,*)
            echo "Invalid waves value: ${WAVES}. Use comma-separated wave numbers, for example: 1,2" >&2
            exit 1
            ;;
    esac
fi

if [ "$SHOW_REASON" = "true" ]; then
    OUTPUT_EXPR='(.target + "\t" + .reason)'
else
    OUTPUT_EXPR='.target'
fi

raw_input=$(cat)
if ! echo "$raw_input" | jq empty 2>/dev/null; then
    echo "Warning: invalid JSON input, returning no targets" >&2
    exit 0
fi
# Extract .repos array from object format, fall back to raw array
input=$(echo "$raw_input" | jq 'if type == "object" then .repos else . end')

echo "$input" | jq -r "
    [ .[] |
      select(.skip != true) |
      select(.checks.config.status == \"ok\") |
      (.wave | tostring) as \$wave |
      select(\$waves == \"\" or ((\",\" + \$waves + \",\") | contains(\",\" + \$wave + \",\"))) |
      select(
        (.wave == 1) or
        (.wave >= 2 and (
          .checks.upstream_sync.status == \"fail\" or
          .checks.dep_sync.status == \"fail\"
        ))
      ) |
      {
        target: ((.repo | split(\"/\") | .[1]) + \"-\" + .branch),
        wave: .wave,
        reason: (if .wave == 1 then \"wave1-always\"
                 elif .checks.upstream_sync.status == \"fail\" then \"upstream-changed\"
                 else \"deps-changed\"
                 end)
      }
    ] | sort_by(.wave) | .[] | ${OUTPUT_EXPR}
" --arg waves "$WAVES"
