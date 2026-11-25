#!/usr/bin/env bash
# Tests for declared-branches.sh: the dashboard's stream list derives from
# the pipeline workflows' oadp_* inputs, never from a hand-kept list.
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/declared-branches.sh"
fail=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

check() { # name expected actual
  if [ "$2" = "$3" ]; then echo "  PASS  $1"; else echo "  FAIL  $1: expected '$2', got '$3'"; fail=1; fi
}

wf() { # dir name inputs...
  local d="$1" n="$2"; shift 2
  mkdir -p "$d"
  { echo "on:"; echo "  workflow_dispatch:"; echo "    inputs:"
    for i in "$@"; do printf '      %s:\n        type: boolean\n' "$i"; done
    echo "jobs: {}"; } > "$d/$n.yaml"
}

# Intersection of both workflows, retired dropped, dev first, newest release next.
wf "$tmp/a" cve-scan oadp_1_3 oadp_1_4 oadp_1_5 oadp_1_6 oadp_1_7 oadp_dev
wf "$tmp/a" auto-rebase-v2 oadp_1_3 oadp_1_4 oadp_1_5 oadp_1_6 oadp_dev
check "intersection, retired dropped, display order" \
  "oadp-dev oadp-1.6 oadp-1.5 oadp-1.4" "$(bash "$script" "$tmp/a")"
check "retired list override" \
  "oadp-dev oadp-1.6 oadp-1.5 oadp-1.4 oadp-1.3" "$(RETIRED_STREAMS='[]' bash "$script" "$tmp/a")"

# 1.10 sorts after 1.9 (version order, not lexical).
wf "$tmp/b" cve-scan oadp_1_9 oadp_1_10 oadp_dev
wf "$tmp/b" auto-rebase-v2 oadp_1_9 oadp_1_10 oadp_dev
check "version sort" "oadp-dev oadp-1.10 oadp-1.9" "$(bash "$script" "$tmp/b")"

# Fail closed: missing workflow, or no inputs at all.
bash "$script" "$tmp/none" > /dev/null 2>&1; check "missing workflow fails" "1" "$?"
wf "$tmp/c" cve-scan
wf "$tmp/c" auto-rebase-v2 oadp_dev
bash "$script" "$tmp/c" > /dev/null 2>&1; check "empty inputs fail" "1" "$?"

# The real workflows produce a non-empty list that includes oadp-dev.
real=$(bash "$script" "$here/../../.github/workflows")
case " $real " in *" oadp-dev "*) check "real workflows include oadp-dev" ok ok ;; *) check "real workflows include oadp-dev" ok "$real" ;; esac

exit "$fail"
