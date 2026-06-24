#!/bin/sh
#
# Tests for DAG bundle generation and assemble-pages integration.
# Verifies the workflow→bundle→pages pipeline without needing GitHub API access.

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

passed=0
failed=0

assert_ok() {
    test_name="$1"
    if [ $? -eq 0 ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        failed=$((failed + 1))
    fi
}

assert_equal() {
    test_name="$1"
    expected="$2"
    actual="$3"
    if [ "$expected" = "$actual" ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    expected: %s\n" "$expected"
        printf "    actual:   %s\n" "$actual"
        failed=$((failed + 1))
    fi
}

assert_file_exists() {
    test_name="$1"
    file="$2"
    if [ -f "$file" ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    file not found: %s\n" "$file"
        failed=$((failed + 1))
    fi
}

# --- Setup ---
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Create sample branch data (mimics rebase-status --json output)
cat > "$TMPDIR/oadp-1.6.json" <<'EOF'
[
  {"repo": "migtools/kopia", "branch": "oadp-1.6", "wave": 1, "skip": false,
   "checks": {"open_pr": {"status": "na", "summary": ""}, "dep_sync": {"status": "ok", "summary": ""}}},
  {"repo": "openshift/velero", "branch": "oadp-1.6", "wave": 2, "skip": false,
   "checks": {"open_pr": {"status": "ok", "summary": "#42"}, "dep_sync": {"status": "ok", "summary": "1/1"}}}
]
EOF

cat > "$TMPDIR/oadp-1.5.json" <<'EOF'
[
  {"repo": "migtools/kopia", "branch": "oadp-1.5", "wave": 1, "skip": false,
   "checks": {"open_pr": {"status": "na", "summary": ""}, "dep_sync": {"status": "ok", "summary": ""}}}
]
EOF

# --- Test: Bundle generation (mimics workflow step) ---
echo "=== Bundle generation ==="

BRANCHES="oadp-1.6 oadp-1.5"
bundle_data="{}"
for branch in $BRANCHES; do
    bundle_data=$(echo "$bundle_data" | jq \
        --arg b "$branch" \
        --slurpfile d "$TMPDIR/${branch}.json" \
        '. + {($b): $d[0]}')
done

default_branch=$(echo "$BRANCHES" | awk '{print $1}')
branches_json=$(echo "$BRANCHES" | tr ' ' '\n' | jq -R . | jq -s .)
echo "$bundle_data" | jq \
    --arg generated_at "2026-06-24T16:00:00Z" \
    --arg default_branch "$default_branch" \
    --argjson branches "$branches_json" \
    '{branches: $branches, generated_at: $generated_at, default_branch: $default_branch, data: .}' \
    > "$TMPDIR/dag-bundle.json"

assert_file_exists "bundle file created" "$TMPDIR/dag-bundle.json"

# Validate structure
assert_equal "has branches array" \
    '["oadp-1.6","oadp-1.5"]' \
    "$(jq -c '.branches' "$TMPDIR/dag-bundle.json")"

assert_equal "has generated_at" \
    "2026-06-24T16:00:00Z" \
    "$(jq -r '.generated_at' "$TMPDIR/dag-bundle.json")"

assert_equal "has default_branch" \
    "oadp-1.6" \
    "$(jq -r '.default_branch' "$TMPDIR/dag-bundle.json")"

assert_equal "data has both branches" \
    '["oadp-1.5","oadp-1.6"]' \
    "$(jq -c '.data | keys' "$TMPDIR/dag-bundle.json")"

assert_equal "oadp-1.6 has 2 repos" \
    "2" \
    "$(jq '.data["oadp-1.6"] | length' "$TMPDIR/dag-bundle.json")"

assert_equal "oadp-1.5 has 1 repo" \
    "1" \
    "$(jq '.data["oadp-1.5"] | length' "$TMPDIR/dag-bundle.json")"

assert_equal "open PR preserved in bundle" \
    "#42" \
    "$(jq -r '.data["oadp-1.6"][1].checks.open_pr.summary' "$TMPDIR/dag-bundle.json")"

# --- Test: assemble-pages picks up bundle ---
echo ""
echo "=== assemble-pages integration ==="

# Set up /tmp/dag-data as the workflow would
rm -rf /tmp/dag-data
mkdir -p /tmp/dag-data
cp "$TMPDIR/dag-bundle.json" /tmp/dag-data/dag-bundle.json

SITE="$TMPDIR/site"
bash "$REPO_ROOT/tools/assemble-pages.sh" "$SITE" > /dev/null

assert_file_exists "bundle in site" "$SITE/rebase-dag/data/dag-bundle.json"
assert_file_exists "index.html in site" "$SITE/rebase-dag/index.html"
assert_file_exists "dag-data.js in site" "$SITE/rebase-dag/dag-data.js"

assert_equal "site bundle matches source" \
    "$(jq -c '.branches' /tmp/dag-data/dag-bundle.json)" \
    "$(jq -c '.branches' "$SITE/rebase-dag/data/dag-bundle.json")"

# --- Test: index.html references dag-bundle.json ---
echo ""
echo "=== index.html contract ==="

if grep -q 'dag-bundle.json' "$REPO_ROOT/tools/rebase-dag/index.html"; then
    printf "  PASS  index.html fetches dag-bundle.json\n"
    passed=$((passed + 1))
else
    printf "  FAIL  index.html does not reference dag-bundle.json\n"
    failed=$((failed + 1))
fi

if grep -q 'bundle\.data\[' "$REPO_ROOT/tools/rebase-dag/index.html"; then
    printf "  PASS  index.html reads bundle.data[branch]\n"
    passed=$((passed + 1))
else
    printf "  FAIL  index.html does not read bundle.data[branch]\n"
    failed=$((failed + 1))
fi

if grep -q 'bundle\.branches' "$REPO_ROOT/tools/rebase-dag/index.html"; then
    printf "  PASS  index.html reads bundle.branches\n"
    passed=$((passed + 1))
else
    printf "  FAIL  index.html does not read bundle.branches\n"
    failed=$((failed + 1))
fi

# --- Summary ---
echo ""
echo "=== Results: $passed passed, $failed failed ==="
[ "$failed" -eq 0 ] || exit 1
