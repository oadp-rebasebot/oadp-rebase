#!/bin/sh
#
# Tests for rebase-decision.sh and rebase-notify.sh

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURES="$SCRIPT_DIR/test-fixtures"
DECISION="$SCRIPT_DIR/rebase-decision.sh"
NOTIFY="$SCRIPT_DIR/rebase-notify.sh"

passed=0
failed=0

assert_output() {
    test_name="$1"
    expected="$2"
    actual="$3"

    if [ "$expected" = "$actual" ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    expected: %s\n" "$(echo "$expected" | head -5)"
        printf "    actual:   %s\n" "$(echo "$actual" | head -5)"
        failed=$((failed + 1))
    fi
}

assert_empty() {
    test_name="$1"
    actual="$2"

    if [ -z "$actual" ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    expected empty, got: %s\n" "$actual"
        failed=$((failed + 1))
    fi
}

assert_contains() {
    test_name="$1"
    haystack="$2"
    needle="$3"

    if echo "$haystack" | grep -qF "$needle"; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    expected to contain: %s\n" "$needle"
        failed=$((failed + 1))
    fi
}

assert_not_contains() {
    test_name="$1"
    haystack="$2"
    needle="$3"

    if ! echo "$haystack" | grep -qF "$needle"; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    expected NOT to contain: %s\n" "$needle"
        failed=$((failed + 1))
    fi
}

assert_json_field() {
    test_name="$1"
    json="$2"
    query="$3"
    expected="$4"

    actual=$(echo "$json" | jq -r "$query")
    if [ "$expected" = "$actual" ]; then
        printf "  PASS  %s\n" "$test_name"
        passed=$((passed + 1))
    else
        printf "  FAIL  %s\n" "$test_name"
        printf "    query: %s\n" "$query"
        printf "    expected: %s\n" "$expected"
        printf "    actual:   %s\n" "$actual"
        failed=$((failed + 1))
    fi
}

# ============================================================
printf "=== rebase-decision.sh tests ===\n\n"
# ============================================================

out=$(bash "$DECISION" < "$FIXTURES/decision/empty.json")
assert_empty "empty input produces no output" "$out"

out=$(bash "$DECISION" < "$FIXTURES/decision/all-skipped.json")
assert_empty "all-skipped produces no output" "$out"

out=$(bash "$DECISION" < "$FIXTURES/decision/wave1-no-pr.json")
assert_output "wave1 without PR is eligible" "kopia-oadp-dev" "$out"

out=$(bash "$DECISION" --reason < "$FIXTURES/decision/wave1-no-pr.json")
assert_contains "wave1 reason is wave1-always" "$out" "wave1-always"

out=$(bash "$DECISION" < "$FIXTURES/decision/wave1-with-pr.json")
assert_empty "wave1 with open PR is skipped" "$out"

out=$(bash "$DECISION" < "$FIXTURES/decision/wave2-deps-fail.json")
assert_output "wave2 with dep_sync=fail is eligible" "velero-oadp-dev" "$out"

out=$(bash "$DECISION" --reason < "$FIXTURES/decision/wave2-deps-fail.json")
assert_contains "wave2 reason is deps-changed" "$out" "deps-changed"

out=$(bash "$DECISION" < "$FIXTURES/decision/wave2-deps-ok.json")
assert_empty "wave2 with dep_sync=ok is skipped" "$out"

out=$(bash "$DECISION" < "$FIXTURES/decision/no-config.json")
assert_empty "repo with config=fail is skipped" "$out"

out=$(bash "$DECISION" < "$FIXTURES/decision/no-rebase.json")
assert_empty "NoRebase repo (config=na) is skipped" "$out"

# Mixed scenario: only oadp-vmdp (wave1, no PR), oadp-operator (deps fail, no PR),
# and oadp-non-admin (deps fail, no PR) should appear.
# Skipped: kopia (has PR), restic (skip=true), velero (has PR),
# hypershift (config=na), velero-plugin-for-aws (has PR).
out=$(bash "$DECISION" < "$FIXTURES/decision/mixed.json")
assert_contains "mixed: wave1 repo without PR included" "$out" "oadp-vmdp-oadp-dev"
assert_contains "mixed: wave3 deps-fail without PR included" "$out" "oadp-operator-oadp-dev"
assert_contains "mixed: wave4 deps-fail without PR included" "$out" "oadp-non-admin-oadp-dev"
assert_not_contains "mixed: wave1 with PR excluded" "$out" "kopia-oadp-dev"
assert_not_contains "mixed: skipped repo excluded" "$out" "restic-oadp-dev"
assert_not_contains "mixed: wave2 with PR excluded" "$out" "velero-oadp-dev"
assert_not_contains "mixed: NoRebase excluded" "$out" "hypershift-oadp-plugin"
assert_not_contains "mixed: deps-fail with PR excluded" "$out" "velero-plugin-for-aws"

# Wave ordering: oadp-vmdp (wave1) should come before oadp-operator (wave3)
first_line=$(echo "$out" | head -1)
assert_output "mixed: wave1 appears first" "oadp-vmdp-oadp-dev" "$first_line"

# ============================================================
printf "\n=== rebase-notify.sh tests ===\n\n"
# ============================================================

# No targets mode
out=$(bash "$NOTIFY" /dev/null --no-targets)
assert_json_field "no-targets: has header block" "$out" '.blocks[0].type' "header"
assert_json_field "no-targets: header text" "$out" '.blocks[0].text.text' "OADP Auto-Rebase"
assert_json_field "no-targets: body mentions no repos" "$out" '.blocks[1].text.text' ":zzz: No repos need rebasing today."

# No targets with dry run
out=$(bash "$NOTIFY" /dev/null --no-targets --dry-run)
assert_json_field "no-targets dry-run: header has label" "$out" '.blocks[0].text.text' "OADP Auto-Rebase [DRY RUN]"

# No targets with run URL
out=$(bash "$NOTIFY" /dev/null --no-targets --run-url "https://example.com/run/1")
assert_json_field "no-targets: context has run URL" "$out" '.blocks[2].elements[0].text' "<https://example.com/run/1|Workflow run>"

# Mixed results
out=$(bash "$NOTIFY" "$FIXTURES/notify-mixed" --run-url "https://example.com/run/1")
body=$(echo "$out" | jq -r '.blocks[1].text.text')
assert_contains "mixed: body has oadp-dev branch" "$body" "*oadp-dev*"
assert_contains "mixed: body has oadp-1.6 branch" "$body" "*oadp-1.6*"
assert_contains "mixed: success icon for velero" "$body" ":white_check_mark:"
assert_contains "mixed: failure icon for kopia-1.6" "$body" ":x:"
assert_contains "mixed: PR link for velero" "$body" "https://github.com/openshift/velero/pull/494"
assert_json_field "mixed: context has summary" "$out" '.blocks[2].elements[0].text' "2/3 succeeded, 1 failed | <https://example.com/run/1|Workflow run>"

# Empty results dir
out=$(bash "$NOTIFY" "$FIXTURES/notify-empty")
assert_json_field "empty results: has header" "$out" '.blocks[0].type' "header"
assert_json_field "empty results: summary shows 0/0" "$out" '.blocks[2].elements[0].text' "0/0 succeeded"

# Valid JSON output
echo "$out" | jq . > /dev/null 2>&1
if [ $? -eq 0 ]; then
    printf "  PASS  all notify outputs are valid JSON\n"
    passed=$((passed + 1))
else
    printf "  FAIL  notify output is not valid JSON\n"
    failed=$((failed + 1))
fi

# ============================================================
printf "\n=== Results ===\n"
# ============================================================

total=$((passed + failed))
printf "%d/%d tests passed\n" "$passed" "$total"

if [ "$failed" -gt 0 ]; then
    printf "%d test(s) FAILED\n" "$failed"
    exit 1
fi

printf "All tests passed.\n"
