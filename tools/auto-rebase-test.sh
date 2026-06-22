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
printf "\n=== conflict-triage tests ===\n\n"
# ============================================================

REBASE_SCRIPT="$SCRIPT_DIR/../run-oadp-rebase.sh"
TRIAGE_FIXTURES="$FIXTURES/conflict-triage"

# --- Script parameterization: no hardcoded --conflict-policy strict ---

# Extract run_local_rebase and run_container_rebase function bodies and check
# for hardcoded strict policy (should only use "$CONFLICT_POLICY")
func_bodies=$(sed -n '/^run_local_rebase()/,/^}/p;/^run_container_rebase()/,/^}/p' "$REBASE_SCRIPT")
hardcoded=$(echo "$func_bodies" | grep -c 'conflict-policy strict' || true)
assert_output "no hardcoded --conflict-policy strict in rebase functions" "0" "$hardcoded"

# Verify $CONFLICT_POLICY is used in both functions
parameterized=$(echo "$func_bodies" | grep -c 'conflict-policy.*CONFLICT_POLICY' || echo "0")
if [ "$parameterized" -ge 2 ]; then
    printf "  PASS  conflict-policy uses \$CONFLICT_POLICY in both functions\n"
    passed=$((passed + 1))
else
    printf "  FAIL  conflict-policy uses \$CONFLICT_POLICY in both functions\n"
    printf "    expected >= 2 occurrences, got: %s\n" "$parameterized"
    failed=$((failed + 1))
fi

# --- Script parameterization: flag is accepted ---

out=$(cd "$SCRIPT_DIR/.." && ./run-oadp-rebase.sh --conflict-policy warn --test nonexistent-target 2>&1 || true)
assert_not_contains "conflict-policy flag is accepted (no 'Unknown option')" "$out" "Unknown option"
assert_contains "conflict-policy flag parsed before config lookup" "$out" "Unknown config"

# --- Script parameterization: default is strict ---

default_val=$(grep '^CONFLICT_POLICY=' "$REBASE_SCRIPT" | head -1 | sed 's/.*="//' | sed 's/"//')
assert_output "default conflict policy is strict" "strict" "$default_val"

# --- Config resolution: workflow grep pattern works for all targets ---

resolve_config() {
    target="$1"
    grep -E "^\s+${target}\)" "$REBASE_SCRIPT" | head -1 | sed 's/.*echo "\(.*\)".*/\1/'
}

assert_output "config resolves: velero-oadp-1.6" \
    "openshift_velero_oadp-1.6" "$(resolve_config velero-oadp-1.6)"
assert_output "config resolves: oadp-vmdp-oadp-1.6" \
    "migtools_oadp_vmdp_oadp-1.6" "$(resolve_config oadp-vmdp-oadp-1.6)"
assert_output "config resolves: kopia-oadp-dev" \
    "migtools_kopia_oadp-dev" "$(resolve_config kopia-oadp-dev)"
assert_output "config resolves: oadp-operator-oadp-dev" \
    "openshift_oadp-operator_oadp-dev" "$(resolve_config oadp-operator-oadp-dev)"
assert_output "config resolves: velero-plugin-for-aws-oadp-1.6" \
    "openshift_velero_plugin_for_aws_oadp-1.6" "$(resolve_config velero-plugin-for-aws-oadp-1.6)"
assert_output "config resolves: oadp-must-gather-oadp-1.6" \
    "openshift_oadp_must_gather_oadp-1.6" "$(resolve_config oadp-must-gather-oadp-1.6)"

# --- Prompt file validation ---

PROMPT_FILE="$SCRIPT_DIR/../.github/prompts/conflict-triage.prompt.yml"

if [ -f "$PROMPT_FILE" ]; then
    printf "  PASS  prompt file exists\n"
    passed=$((passed + 1))
else
    printf "  FAIL  prompt file exists\n"
    failed=$((failed + 1))
fi

if ruby -ryaml -e "YAML.load_file('$PROMPT_FILE')" 2>/dev/null; then
    printf "  PASS  prompt file is valid YAML\n"
    passed=$((passed + 1))
else
    printf "  FAIL  prompt file is valid YAML\n"
    failed=$((failed + 1))
fi

prompt_content=$(cat "$PROMPT_FILE")
assert_contains "prompt has rebase_output variable" "$prompt_content" "{{rebase_output}}"
assert_contains "prompt has hook_config variable" "$prompt_content" "{{hook_config}}"

# --- Test fixture validation ---

safe_fixture="$TRIAGE_FIXTURES/safe-gomod-only.txt"
unsafe_fixture="$TRIAGE_FIXTURES/unsafe-code-file.txt"

safe_warnings=$(grep -c '^WARNING' "$safe_fixture" || echo "0")
if [ "$safe_warnings" -gt 0 ]; then
    printf "  PASS  safe fixture has WARNING lines\n"
    passed=$((passed + 1))
else
    printf "  FAIL  safe fixture has WARNING lines\n"
    failed=$((failed + 1))
fi

# Safe fixture: all WARNING "dropped from" lines should only reference go.mod or go.sum
unsafe_files_in_safe=$(grep '^WARNING.*dropped from' "$safe_fixture" | grep -v "'go\.mod'" | grep -v "'go\.sum'" || true)
assert_empty "safe fixture only has go.mod/go.sum warnings" "$unsafe_files_in_safe"

# Unsafe fixture: should have warnings about non-go.mod files
assert_contains "unsafe fixture has .go file warning" "$(cat "$unsafe_fixture")" "restore.go"
assert_contains "unsafe fixture has downstream-only file warning" "$(cat "$unsafe_fixture")" "Dockerfile.ubi"

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
