#!/usr/bin/env bash
# Print the live OADP stream branches, space-separated: oadp-dev first, then
# the release branches newest first (the dashboard's display order).
#
# Source of truth: the oadp_* boolean inputs that BOTH pipeline workflows
# (cve-scan.yaml, auto-rebase-v2.yaml) declare -- the same inputs stream-sync
# regenerates and chai-trigger derives its allowlist from -- minus retired
# streams. Adding oadp-1.7 upstream needs no edit to any consumer.
#
# Usage: declared-branches.sh [workflows_dir]
#   RETIRED_STREAMS  JSON array of retired branch names (default: ["oadp-1.3"])
#
# Fails closed (exit 1, nothing on stdout) when either workflow is missing or
# declares no oadp_* inputs: a silently empty list would publish an empty
# dashboard.
set -euo pipefail

dir="${1:-.github/workflows}"
retired="${RETIRED_STREAMS:-[\"oadp-1.3\"]}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for wf in cve-scan auto-rebase-v2; do
  f="$dir/$wf.yaml"
  [ -s "$f" ] || { echo "declared-branches: $f missing or empty" >&2; exit 1; }
  sed -n '/^  workflow_dispatch:/,/^run-name\|^permissions:\|^jobs:/p' "$f" \
    | sed -n 's/^      \(oadp_[a-z0-9_]*\):$/\1/p' | sort -u > "$tmp/$wf"
  [ -s "$tmp/$wf" ] || { echo "declared-branches: no oadp_* inputs in $f" >&2; exit 1; }
done

out=$(comm -12 "$tmp/cve-scan" "$tmp/auto-rebase-v2" \
  | sed 's/^oadp_//; s/_/./g; s/^/oadp-/' \
  | while IFS= read -r b; do
      jq -e --arg b "$b" 'index($b) != null' <<< "$retired" > /dev/null || echo "$b"
    done \
  | sort -V -r \
  | awk '$0 == "oadp-dev" { dev = 1; next } { rel = rel " " $0 } END { printf "%s%s\n", (dev ? "oadp-dev" : ""), rel }' \
  | sed 's/^ //')

[ -n "$out" ] || { echo "declared-branches: no live streams declared" >&2; exit 1; }
echo "$out"
