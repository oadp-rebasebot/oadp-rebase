#!/bin/bash
# validate-payload.sh — the trigger-comment JSON schema, executable.
#
# Usage: validate-payload.sh <payload-json> <allowed-branches-json>
#   e.g. validate-payload.sh "$PAYLOAD" "$ALLOWED_BRANCHES"
# Exit 0 = valid; exit 1 = invalid (reason on stderr).
#
# This file is the schema contract chai validates against before posting
# (ship-help-bot tools/github_workflow_dispatch), the workflow validates
# against after receiving, and tools/chai-trigger/test-schema.sh keeps
# honest against the workflows' declared inputs. Change it in one commit
# with: cve-scan.yaml and auto-rebase-v2.yaml boolean inputs, and the
# chai-side config (see the ALLOWED_BRANCHES comment in the workflow).
set -euo pipefail

PAYLOAD="${1:?payload json required}"
ALLOWED="${2:?allowed branches json required}"

echo "$PAYLOAD" | jq -e --argjson allowed "$ALLOWED" '
  type == "object"
  # schema.json sets additionalProperties: false; enforce it here too so
  # the executable form and the declarative form refuse the same payloads.
  and ((keys | sort) - ["branches","dry_run","plan_id","timeout_minutes","trigger","waves"] == [])
  and (.trigger == "cve_scan" or .trigger == "rebase" or .trigger == "rebase_then_cve")
  and ((.waves // "") | test("^[1-5]$"))
  and (.dry_run == true or .dry_run == false)
  and (.branches | type == "array" and length == 1)
  and (.branches | all(. as $b | $allowed | index($b)))
  and ((.timeout_minutes // 120) | (type == "number" and . >= 10 and . <= 360))
  and ((.plan_id // "") | (type == "string" and length <= 64))
' > /dev/null 2>&1 || {
  echo "payload does not satisfy the chai-trigger schema" >&2
  exit 1
}
exit 0
