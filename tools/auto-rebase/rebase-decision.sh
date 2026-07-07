#!/bin/sh
#
# Reads rebase-status --json from stdin and outputs repo-branch targets
# that are eligible for a rebasebot run.
#
# Usage:
#   rebase-status --json --hide-dependency-details oadp-dev | rebase-decision.sh [--reason]
#
# Decision logic per repo:
#   1. Skip if skip == true
#   2. Skip if checks.config.status != "ok" (no config or NoRebase)
#   3. Wave 1: always eligible (no internal deps)
#   4. Wave 2+: eligible when checks.dep_sync.status == "fail" (deps moved ahead)
#
# Output: one target per line, sorted by wave, e.g. "velero-oadp-dev"
#   --reason: append tab-separated reason (wave1-always | deps-changed)

set -eu

SHOW_REASON=false

for arg in "$@"; do
    case "$arg" in
        --reason) SHOW_REASON=true ;;
        -h|--help)
            cat <<'USAGE'
Reads rebase-status --json from stdin and outputs repo-branch targets
that are eligible for a rebasebot run.

Usage:
  rebase-status --json --hide-dependency-details oadp-dev | rebase-decision.sh [--reason]

Options:
  --reason    Append tab-separated reason (wave1-always | deps-changed)
  -h, --help  Show this help
USAGE
            exit 0
            ;;
        *) echo "Unknown option: $arg" >&2; exit 1 ;;
    esac
done

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
      select(
        (.wave == 1) or
        (.wave >= 2 and .checks.dep_sync.status == \"fail\")
      ) |
      {
        target: ((.repo | split(\"/\") | .[1]) + \"-\" + .branch),
        wave: .wave,
        reason: (if .wave == 1 then \"wave1-always\" else \"deps-changed\" end)
      }
    ] | sort_by(.wave) | .[] | ${OUTPUT_EXPR}
"
