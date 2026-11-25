#!/bin/bash
# test-tide-sync.sh — keeps the openshift/release tide queries honest.
#
# Every stream the sibling workflows declare (their oadp_* boolean inputs)
# must appear in the includedBranches of every author-scoped
# oadp-rebasebot-app[bot] tide query in openshift/release. When stream-sync
# adds a new boolean, this fails until the matching release PR merges —
# the alarm for "tide queries lag the declared streams".
#
# Run by `make tide-sync-test` (CI: config-tests.yaml).
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
FAILURES=0
fail() { echo "FAIL: $*" >&2; FAILURES=$((FAILURES + 1)); }

# The declared set, derived from the sibling booleans (same source the
# workflows derive from — no separate list to maintain here), minus the
# retired streams: T32 may drop oadp-1.3 from the tide queries while the
# dispatch-UI boolean stays, and that must not fail this test.
RETIRED='["oadp-1.3"]'
declare -a DECLARED=()
for wf in cve-scan auto-rebase-v2; do
  while IFS= read -r key; do
    branch="oadp-$(sed 's/^oadp_//; s/_/./g' <<< "$key")"
    if ! echo "$RETIRED" | jq -e --arg b "$branch" 'index($b) != null' > /dev/null; then
      DECLARED+=("$branch")
    fi
  done < <(sed -n '/^  workflow_dispatch:/,/^run-name\|^permissions:/p' "$ROOT/.github/workflows/$wf.yaml" \
    | sed -n 's/^      \(oadp_[a-z0-9_]*\):$/\1/p' | sort -u)
done
mapfile -t DECLARED < <(printf '%s\n' "${DECLARED[@]}" | sort -u)
[ "${#DECLARED[@]}" -gt 0 ] || { echo "FAIL: derived an empty declared set"; exit 1; }

# Same retired list as chai-trigger.yaml (test-schema.sh guards its own
# copy the same way): three copies must never drift.
WF_RETIRED=$(sed -n "s/^      RETIRED_STREAMS: '\(.*\)'$/\1/p" "$ROOT/.github/workflows/chai-trigger.yaml")
[ -n "$WF_RETIRED" ] || { echo "FAIL: could not read RETIRED_STREAMS from chai-trigger.yaml"; exit 1; }
[ "$WF_RETIRED" = "$RETIRED" ] \
  || { echo "FAIL: test RETIRED ($RETIRED) != chai-trigger.yaml RETIRED_STREAMS ($WF_RETIRED)"; exit 1; }
echo "declared streams: ${DECLARED[*]}"

# Sparse-checkout only the prow config tree; find the author-scoped queries
# by grep — no hardcoded repo list.
DIR=$(mktemp -d)
trap 'rm -rf "$DIR"' EXIT
git clone -q --depth 1 --filter=blob:none --sparse https://github.com/openshift/release.git "$DIR/release"
git -C "$DIR/release" sparse-checkout set core-services/prow/02_config

# Real YAML parsing (field order, quoted values, excludedBranches all
# handled by the parser, not by line shapes): each candidate file is
# verified by verify-tide-queries.py, which reports MISSING branches,
# NO-QUERY (a candidate file with no parseable author-scoped query --
# itself a failure, format drift), and a COUNTED line per file. A
# global total alone hid a bad file behind a good one, so every
# candidate file must COUNT at least one query.
VERIFIED=0
while IFS= read -r f; do
  if out=$(python3 "$(dirname "$0")/verify-tide-queries.py" "${DECLARED[@]}" < "$f"); then
    VERIFIED=$((VERIFIED + $(grep -o '[0-9]\+$' <<< "$out" | tail -1)))
  else
    while IFS= read -r line; do
      case "$line" in
        "MISSING: "*) fail "$f: author-scoped tide query is missing branch ${line#MISSING: } in includedBranches" ;;
        "NO-QUERY: "*) fail "$f: ${line#NO-QUERY: }" ;;
        *) fail "$f: $line" ;;
      esac
    done <<< "$out"
  fi
done < <(grep -rl 'oadp-rebasebot-app\[bot\]' "$DIR/release/core-services/prow/02_config" --include='_prowconfig.yaml')

[ "$VERIFIED" -gt 0 ] || fail "no author-scoped tide query was VERIFIED (grep matched files but no query parsed -- format drift?)"

if [ "$FAILURES" -gt 0 ]; then
  echo "$FAILURES tide-sync check(s) failed" >&2
  exit 1
fi
echo "$VERIFIED tide queries carry every declared stream"
