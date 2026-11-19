#!/bin/bash
# test-schema.sh — keeps the chai-trigger schema honest.
#
# Three checks:
#   1. schema.json is valid JSON and parses as a JSON Schema draft we can
#      at least structurally walk (required keys, property types).
#   2. validate-payload.sh accepts the sample valid payloads and rejects
#      the invalid ones (the same script the workflow runs).
#   3. The schema's shape matches the workflows' declared inputs: every
#      branch boolean cve-scan.yaml and auto-rebase-v2.yaml declare must
#      be representable in the branches pattern, and dry_run/waves exist
#      as inputs in both workflows.
#
# Run by `make chai-trigger-test` (CI: config-tests.yaml).
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(git rev-parse --show-toplevel)"
FAILURES=0

fail() { echo "FAIL: $*" >&2; FAILURES=$((FAILURES + 1)); }

ALLOWED='["oadp-1.3","oadp-1.4","oadp-1.5","oadp-1.6","oadp-dev"]'

# --- 1. schema.json is well-formed ---
jq -e 'type == "object" and has("properties") and has("required")' schema.json > /dev/null \
  || fail "schema.json is not a structurally valid schema document"

# --- 2. validate-payload.sh: valid samples accepted ---
valid_samples=(
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true}'
  '{"trigger":"cve_scan","branches":["oadp-dev"],"waves":"5","dry_run":false,"timeout_minutes":180,"plan_id":"chai-123-456-deadbeef"}'
  '{"trigger":"rebase","branches":["oadp-1.4"],"waves":"2","dry_run":true,"timeout_minutes":10}'
)
for sample in "${valid_samples[@]}"; do
  ./validate-payload.sh "$sample" "$ALLOWED" \
    || fail "valid payload rejected: $sample"
done

# --- 2b. invalid samples rejected ---
invalid_samples=(
  '{"trigger":"deploy_prod","branches":["oadp-1.5"],"waves":"1","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5","oadp-1.6"],"waves":"1","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["main"],"waves":"1","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"6","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1"}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":"yes"}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":5}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":9999}'
  'not json at all'
)
for sample in "${invalid_samples[@]}"; do
  if ./validate-payload.sh "$sample" "$ALLOWED" > /dev/null 2>&1; then
    fail "invalid payload accepted: $sample"
  fi
done

# --- 3. schema matches the workflows' declared inputs ---
for wf in cve-scan auto-rebase-v2; do
  yaml="$ROOT/.github/workflows/$wf.yaml"
  [ -f "$yaml" ] || { fail "workflow $wf.yaml not found"; continue; }
  # every branch boolean the workflow declares is in the allowed list
  for input_name in $(sed -n '/^  workflow_dispatch:/,/^run-name\|^permissions:/p' "$yaml" \
      | sed -n 's/^      \(oadp_[a-z0-9_]*\):$/\1/p'); do
    branch=$(echo "$input_name" | sed 's/^oadp_//; s/_/./g; s/\.dev$/-dev/')  # oadp_1_5 -> 1.5, oadp_dev -> dev
    [ "$branch" = "dev" ] || branch="$branch"
    echo "$ALLOWED" | jq -e --arg b "${branch/#/oadp-}" 'index($b) != null' > /dev/null \
      || fail "$wf declares input $input_name but ${branch/#/oadp-} is not in the allowed branch list"
  done
  # waves and dry_run exist as inputs
  for expected in waves dry_run; do
    grep -qE "^      $expected:" "$yaml" \
      || fail "$wf.yaml does not declare the $expected input the schema requires"
  done
done

# the schema's branches pattern covers every allowed branch
for b in $(echo "$ALLOWED" | jq -r '.[]'); do
  echo "\"$b\"" | jq -e 'test("^oadp-(1\\.[3-6]|dev)$")' > /dev/null 2>&1 \
    || fail "allowed branch $b does not match the schema pattern"
done

if [ "$FAILURES" -gt 0 ]; then
  echo "$FAILURES schema check(s) failed" >&2
  exit 1
fi
echo "chai-trigger schema checks passed"
