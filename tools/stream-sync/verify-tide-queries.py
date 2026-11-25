#!/usr/bin/env python3
"""Per-file tide-query verification for test-tide-sync.sh.

Reads each candidate _prowconfig.yaml with a real YAML parser and
verifies EVERY author-scoped query's includedBranches carries every
declared stream. Handles any field order (author inline or later,
quoted or bare), ignores excludedBranches, and reports per file:
zero parsed author-scoped queries in a candidate file is itself a
failure (format drift), not a pass.

Usage: verify-tide-queries.py <declared-branch>... < file
Exit 0 = all queries carry every declared branch (COUNTED lines on
stdout); nonzero = MISSING lines / NO-QUERY lines on stdout.
"""
import sys
import yaml

declared = [sys.argv[1]] if len(sys.argv) == 2 else sys.argv[1:]

doc = yaml.safe_load(sys.stdin.read())
queries = ((doc or {}).get("tide") or {}).get("queries") or []
if not isinstance(queries, list):
    print("NO-QUERY: tide.queries is not a list")
    sys.exit(1)

ours = [q for q in queries
        if isinstance(q, dict) and q.get("author") == "oadp-rebasebot-app[bot]"]
if not ours:
    print("NO-QUERY: no author-scoped tide query parsed")
    sys.exit(1)

failures = 0
for q in ours:
    included = q.get("includedBranches")
    if not isinstance(included, list) or not included:
        print("NO-QUERY: author-scoped query has no includedBranches list")
        failures += 1
        continue
    for b in declared:
        if b not in included:
            print(f"MISSING: {b}")
            failures += 1
print(f"COUNTED: {len(ours)}")
sys.exit(1 if failures else 0)
