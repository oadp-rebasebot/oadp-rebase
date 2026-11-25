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

# The accepted set is derived the same way chai-trigger.yaml derives it at
# run time: the oadp_* booleans BOTH sibling workflows declare (read locally
# -- CI has the checkout), minus the retired streams. Adding oadp_1_7 to the
# siblings needs no edit here.
RETIRED='["oadp-1.3"]'
derive_allowed() {
  for wf in cve-scan auto-rebase-v2; do
    sed -n '/^  workflow_dispatch:/,/^run-name\|^permissions:/p' "$ROOT/.github/workflows/$wf.yaml" \
      | sed -n 's/^      \(oadp_[a-z0-9_]*\):$/\1/p' | sort > "/tmp/${wf}.inputs"
  done
  for input_name in $(comm -12 /tmp/cve-scan.inputs /tmp/auto-rebase-v2.inputs); do
    branch=$(sed 's/^oadp_//; s/_/./g' <<< "$input_name")
    if ! echo "$RETIRED" | jq -e --arg b "oadp-${branch}" 'index($b) != null' > /dev/null; then
      echo "oadp-${branch}"
    fi
  done | jq -R -s -c 'split("\n") | map(select(length > 0))'
}
ALLOWED=$(derive_allowed)
[ "$ALLOWED" != "[]" ] || { echo "FAIL: derived an empty allowed set"; exit 1; }
echo "derived allowed set: $ALLOWED"

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
  '{"trigger":"rebase_then_cve","branches":["oadp-1.3"],"waves":"1","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"6","dry_run":true}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1"}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":"yes"}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":5}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":9999}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"foo":"bar"}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":90.5}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"plan_id":"x\"; id; echo \""}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"plan_id":"two words"}'
  '{"trigger":"cve_scan","branches":["oadp-1.3"],"waves":"9","dry_run":true}{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true}'
  '{"trigger":"cve_scan","branches":[["oadp-1.5","oadp-1.6"]],"waves":"1","dry_run":true}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"plan_id":null}'
  '{"trigger":"rebase_then_cve","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":340}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":false}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":null}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"plan_id":false}'
  '{"trigger":"cve_scan","branches":["oadp-1.5"],"waves":"1","dry_run":true,"timeout_minutes":120,"plan_id":null}'
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
  # every allowed branch is dispatchable: the workflow declares its boolean.
  # The reverse need not hold: a workflow may declare a boolean for a stream
  # the comment path refuses (oadp-1.3 left support 2026-10-31) -- its own
  # dispatch UI can still use it.
  for b in $(echo "$ALLOWED" | jq -r '.[]'); do
    key=$(tr '.-' '__' <<< "$b")
    grep -qE "^      $key:" "$yaml" \
      || fail "$wf does not declare the $key input the allowed branch $b needs"
  done
  # waves, dry_run and plan_id exist as inputs
  for expected in waves dry_run plan_id; do
    grep -qE "^      $expected:" "$yaml" \
      || fail "$wf.yaml does not declare the $expected input the schema requires"
  done
done

# the schema's branches pattern (read from schema.json, never restated here)
# covers every allowed branch
PATTERN=$(jq -r '.properties.branches.items.pattern' schema.json)
for b in $(echo "$ALLOWED" | jq -r '.[]'); do
  # -e: a `false` output must FAIL the check -- a plain -rn exits 0 on
  # false and the assertion never fires.
  jq -ern --arg b "$b" --arg p "$PATTERN" '$b | test($p)' > /dev/null 2>&1 \
    || fail "allowed branch $b does not match the schema pattern $PATTERN"
done
# self-test: a non-matching branch must fail the same assertion.
if jq -ern --arg b "oadp-9.9" --arg p "$PATTERN" '$b | test($p)' > /dev/null 2>&1; then
  fail "the pattern assertion passes a non-matching branch -- the test is vacuous"
fi

# the workflow derives its set the same way this test does -- guard the
# derivation's inputs instead: both siblings declare every allowed branch,
# and the retired list in chai-trigger.yaml matches this one.
for b in $(echo "$ALLOWED" | jq -r '.[]'); do
  key=$(tr '.-' '__' <<< "$b")
  for wf in cve-scan auto-rebase-v2; do
    grep -qE "^      $key:" "$ROOT/.github/workflows/$wf.yaml" \
      || fail "$wf does not declare the $key input the allowed branch $b needs"
  done
done
WF_RETIRED=$(sed -n "s/^      RETIRED_STREAMS: '\(.*\)'$/\1/p" "$ROOT/.github/workflows/chai-trigger.yaml")
[ -n "$WF_RETIRED" ] || fail "could not read RETIRED_STREAMS from chai-trigger.yaml"
[ "$WF_RETIRED" = "$RETIRED" ] \
  || fail "test RETIRED ($RETIRED) != chai-trigger.yaml RETIRED_STREAMS ($WF_RETIRED)"

if [ "$FAILURES" -gt 0 ]; then
  echo "$FAILURES schema check(s) failed" >&2
  exit 1
fi
echo "chai-trigger schema checks passed"
