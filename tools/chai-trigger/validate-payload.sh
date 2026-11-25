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

# -s (slurp) wraps the input in an array: exactly ONE JSON object is
# accepted. A bare filter over concatenated objects takes its exit status
# from the last output only, so `{"invalid":1}{"valid":...}` would pass.
echo "$PAYLOAD" | jq -s -e --argjson allowed "$ALLOWED" '
  length == 1 and (.[0]
  | type == "object"
  # schema.json sets additionalProperties: false; enforce it here too so
  # the executable form and the declarative form refuse the same payloads.
  and ((keys | sort) - ["branches","dry_run","plan_id","timeout_minutes","trigger","waves"] == [])
  and (.trigger == "cve_scan" or .trigger == "rebase" or .trigger == "rebase_then_cve")
  # waves as an exact-string enumeration: a regex $ can match before a
  # trailing newline, and `// ""` treats null/false as absent.
  and (has("waves") and (.waves | type == "string")
       and (.waves == "1" or .waves == "2" or .waves == "3" or .waves == "4" or .waves == "5"))
  and (.dry_run == true or .dry_run == false)
  and (.branches | type == "array" and length == 1)
  # items must be strings: an array item makes index() search for a
  # sub-array and the payload slip past the allowlist
  and (.branches | all(type == "string"))
  and (.branches | all(. as $b | $allowed | index($b)))
  # 330, not 360: GitHub-hosted runners hard-kill a job at 360 minutes,
  # and that kill is a cancel -- failure() never fires, the rocket stays
  # up with no verdict. The budget plus pipeline overhead must fit.
  # The default applies only when ABSENT: null/false must not slip
  # through as "unset".
  and ((has("timeout_minutes") and (.timeout_minutes | type != "number")) | not)
  and ((.timeout_minutes // 120) | (type == "number" and . == floor and . >= 10 and . <= 330))
  # plan_id is interpolated into the dispatch step shell and echoed into
  # GITHUB_OUTPUT: a conservative charset kills injection (quotes,
  # semicolons, newlines) at the schema, before any workflow step sees it.
  # plan_id: absent is fine; present must be a string (null/false are not)
  and ((has("plan_id") and (.plan_id | type != "string")) | not)
  and ((.plan_id // "") | (type == "string" and (. == "" or (length <= 64 and test("^[A-Za-z0-9._-]+$")))))
  )
' > /dev/null 2>&1 || {
  echo "payload does not satisfy the chai-trigger schema" >&2
  exit 1
}
exit 0
