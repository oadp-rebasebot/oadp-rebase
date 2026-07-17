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
assert_output "wave1 with open PR is eligible" "kopia-oadp-dev" "$out"

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

# Mixed scenario: kopia (wave1), oadp-vmdp (wave1), velero-plugin-for-aws (wave3, deps fail),
# oadp-operator (wave3, deps fail), and oadp-non-admin (wave4, deps fail) should appear.
# Open PRs no longer cause skipping — rebasebot updates existing PRs.
# Skipped: restic (skip=true), velero (wave2, deps in sync), hypershift (config=na).
out=$(bash "$DECISION" < "$FIXTURES/decision/mixed.json")
assert_contains "mixed: wave1 repo included" "$out" "kopia-oadp-dev"
assert_contains "mixed: wave1 repo included" "$out" "oadp-vmdp-oadp-dev"
assert_contains "mixed: wave3 deps-fail included" "$out" "oadp-operator-oadp-dev"
assert_contains "mixed: wave3 deps-fail with PR included" "$out" "velero-plugin-for-aws-oadp-dev"
assert_contains "mixed: wave4 deps-fail included" "$out" "oadp-non-admin-oadp-dev"
assert_not_contains "mixed: skipped repo excluded" "$out" "restic-oadp-dev"
assert_not_contains "mixed: wave2 deps-ok excluded" "$out" "velero-oadp-dev"
assert_not_contains "mixed: NoRebase excluded" "$out" "hypershift-oadp-plugin"

# Wave ordering: wave1 repos should come before wave3+
first_line=$(echo "$out" | head -1)
assert_output "mixed: wave1 appears first" "kopia-oadp-dev" "$first_line"

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

REBASE_SCRIPT="$SCRIPT_DIR/../../run-oadp-rebase.sh"
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

out=$(cd "$SCRIPT_DIR/../.." && ./run-oadp-rebase.sh --conflict-policy warn --test nonexistent-target 2>&1 || true)
assert_not_contains "conflict-policy flag is accepted (no 'Unknown option')" "$out" "Unknown option"
assert_contains "conflict-policy flag parsed before config lookup" "$out" "Unknown config"

# --- Script parameterization: default is strict ---

default_val=$(grep '^CONFLICT_POLICY=' "$REBASE_SCRIPT" | head -1 | sed 's/.*="//' | sed 's/"//')
assert_output "default conflict policy is strict" "strict" "$default_val"

# --- Config resolution: yq-based lookup works for all targets ---

resolve_config() {
    target="$1"
    repo_name=$(echo "$target" | sed -E 's/-(oadp-dev|oadp-1\.[0-9]+|main)$//')
    branch=$(echo "$target" | sed "s/^${repo_name}-//")
    prefix=$(yq -r ".repos[] | select(.repo == \"${repo_name}\") | .config_prefix" "$SCRIPT_DIR/../../repos.yaml")
    [ -z "$prefix" ] || [ "$prefix" = "null" ] && return 1
    echo "${prefix}_${branch}"
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

# --- conflict-triage.sh tests ---

TRIAGE_SCRIPT="$SCRIPT_DIR/conflict-triage.sh"
safe_fixture="$TRIAGE_FIXTURES/safe-gomod-only.txt"
unsafe_fixture="$TRIAGE_FIXTURES/unsafe-code-file.txt"
config_with_tidy="$TRIAGE_FIXTURES/hook-config-with-tidy.env.sh"
config_without_tidy="$TRIAGE_FIXTURES/hook-config-without-tidy.env.sh"

# Safe fixture + hooks that cover go.mod → safe verdict
safe_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" < "$safe_fixture") && safe_rc=0 || safe_rc=$?
assert_output "safe fixture with go-mod-tidy hook → exit 0" "0" "$safe_rc"
assert_json_field "safe verdict is true" "$safe_verdict" '.safe' "true"
assert_json_field "safe verdict has reason" "$safe_verdict" '.reason' "All conflicting files are covered by configured hooks"

# Unsafe fixture → unsafe verdict (has .go and Dockerfile.ubi warnings)
unsafe_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" < "$unsafe_fixture") && unsafe_rc=0 || unsafe_rc=$?
assert_output "unsafe fixture → exit 1" "1" "$unsafe_rc"
assert_json_field "unsafe verdict is false" "$unsafe_verdict" '.safe' "false"
assert_contains "unsafe reason mentions uncovered file" "$unsafe_verdict" "not covered by any configured hook"

# Safe fixture but hooks don't include go-mod-tidy → unsafe (hook not configured)
no_tidy_verdict=$(bash "$TRIAGE_SCRIPT" "$config_without_tidy" < "$safe_fixture") && no_tidy_rc=0 || no_tidy_rc=$?
assert_output "safe fixture without go-mod-tidy hook → exit 1" "1" "$no_tidy_rc"
assert_json_field "missing hook verdict is false" "$no_tidy_verdict" '.safe' "false"

# No conflict-policy error in output (e.g. image pull failure) → unsafe, don't retry
infra_verdict=$(echo "Error: copying system image from manifest list: unexpected EOF" | bash "$TRIAGE_SCRIPT" "$config_with_tidy") && infra_rc=0 || infra_rc=$?
assert_output "infrastructure failure → exit 1" "1" "$infra_rc"
assert_json_field "infra failure verdict is false" "$infra_verdict" '.safe' "false"
assert_contains "infra failure reason" "$infra_verdict" "did not fail due to conflict policy"

# Verify JSON structure has affected_files array
file_count=$(echo "$safe_verdict" | jq '.affected_files | length')
if [ "$file_count" -gt 0 ]; then
    printf "  PASS  verdict has affected_files array (%s files)\n" "$file_count"
    passed=$((passed + 1))
else
    printf "  FAIL  verdict has affected_files array\n"
    failed=$((failed + 1))
fi

# --- expected_conflicts allowlist tests ---

expected_fixture="$TRIAGE_FIXTURES/safe-expected-conflicts.txt"
partial_fixture="$TRIAGE_FIXTURES/unsafe-partial-allowlist.txt"

# Allowlisted files (cli/app.go, repo/blob/sftp/sftp_storage_test.go) + go.mod → safe with repo name
ec_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" "oadp-vmdp" < "$expected_fixture") && ec_rc=0 || ec_rc=$?
assert_output "expected_conflicts: allowlisted files + go.mod → exit 0" "0" "$ec_rc"
assert_json_field "expected_conflicts: verdict is safe" "$ec_verdict" '.safe' "true"

# Verify the allowlisted files show covered_by expected_conflicts
ec_cli_hook=$(echo "$ec_verdict" | jq -r '.affected_files[] | select(.file == "cli/app.go") | .hook_name')
assert_output "expected_conflicts: cli/app.go covered by expected_conflicts" "expected_conflicts" "$ec_cli_hook"

ec_sftp_hook=$(echo "$ec_verdict" | jq -r '.affected_files[] | select(.file == "repo/blob/sftp/sftp_storage_test.go") | .hook_name')
assert_output "expected_conflicts: sftp test covered by expected_conflicts" "expected_conflicts" "$ec_sftp_hook"

# go.mod should still be covered by go-mod-tidy hook (not expected_conflicts)
ec_gomod_hook=$(echo "$ec_verdict" | jq -r '.affected_files[] | select(.file == "go.mod") | .hook_name')
assert_output "expected_conflicts: go.mod still covered by hook" "go-mod-tidy-and-commit.sh" "$ec_gomod_hook"

# Same fixture without repo name → unsafe (no allowlist loaded)
ec_norepo_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" < "$expected_fixture") && ec_norepo_rc=0 || ec_norepo_rc=$?
assert_output "expected_conflicts: without repo name → exit 1" "1" "$ec_norepo_rc"
assert_json_field "expected_conflicts: without repo name → unsafe" "$ec_norepo_verdict" '.safe' "false"

# Partial allowlist: cli/app.go is allowed but internal/server/api.go is not → unsafe
partial_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" "oadp-vmdp" < "$partial_fixture") && partial_rc=0 || partial_rc=$?
assert_output "expected_conflicts: partial allowlist → exit 1" "1" "$partial_rc"
assert_json_field "expected_conflicts: partial allowlist → unsafe" "$partial_verdict" '.safe' "false"
assert_contains "expected_conflicts: reason mentions uncovered file" "$partial_verdict" "internal/server/api.go"

# Repo with no expected_conflicts → allowlist is empty, no effect
ec_empty_verdict=$(bash "$TRIAGE_SCRIPT" "$config_with_tidy" "velero" < "$expected_fixture") && ec_empty_rc=0 || ec_empty_rc=$?
assert_output "expected_conflicts: repo without allowlist → exit 1" "1" "$ec_empty_rc"
assert_json_field "expected_conflicts: repo without allowlist → unsafe" "$ec_empty_verdict" '.safe' "false"

# Verify repos.yaml has expected_conflicts for oadp-vmdp (SSOT check)
ec_count=$(yq -r '.repos[] | select(.repo == "oadp-vmdp") | .expected_conflicts | length' "$SCRIPT_DIR/../../repos.yaml")
assert_output "repos.yaml: oadp-vmdp has 2 expected_conflicts" "2" "$ec_count"

# ============================================================
printf "\n=== must-gather submodule hook tests ===\n\n"
# ============================================================

HOOK_DIR="$SCRIPT_DIR/../../rebasebot-hook-scripts"
CONFIG_DIR="$SCRIPT_DIR/../../rebase-configs"

# --- Hook scripts exist ---

for variant in oadp-1.6 oadp-dev; do
    for sub in velero kopia; do
        hook="$HOOK_DIR/${sub}-submodule-and-commit_${variant}.sh"
        if [ -f "$hook" ]; then
            printf "  PASS  ${sub}-submodule hook exists for ${variant}\n"
            passed=$((passed + 1))
        else
            printf "  FAIL  ${sub}-submodule hook exists for ${variant}\n"
            printf "    expected: %s\n" "$hook"
            failed=$((failed + 1))
        fi
    done
    # restic hook should already exist
    hook="$HOOK_DIR/restic-submodule-and-commit_${variant}.sh"
    if [ -f "$hook" ]; then
        printf "  PASS  restic-submodule hook exists for ${variant}\n"
        passed=$((passed + 1))
    else
        printf "  FAIL  restic-submodule hook exists for ${variant}\n"
        failed=$((failed + 1))
    fi
done

# --- Must-gather configs include all 3 submodule hooks ---

for variant in oadp-1.6 oadp-dev; do
    config="$CONFIG_DIR/openshift_oadp_must_gather_${variant}.env.sh"
    config_content=$(cat "$config")
    for sub in velero restic kopia; do
        assert_contains "must-gather ${variant} has ${sub}-submodule hook" \
            "$config_content" "${sub}-submodule-and-commit_${variant}.sh"
    done
done

# ============================================================
printf "\n=== JSON object format compatibility tests ===\n\n"
# ============================================================

# rebase-status --json now outputs {"velero_tag_alignment": {...}, "repos": [...]}
# instead of a bare array. All consumers must handle both formats.

NOTIFY_STATUS="$SCRIPT_DIR/rebase-status-notify.sh"

# --- rebase-decision.sh accepts object format ---

out=$(bash "$DECISION" < "$FIXTURES/decision/mixed-object.json")
assert_contains "decision: object format includes wave1 target" "$out" "oadp-vmdp-oadp-dev"
assert_contains "decision: object format includes wave1 with PR target" "$out" "kopia-oadp-dev"
assert_contains "decision: object format includes wave3 target" "$out" "oadp-operator-oadp-dev"

# Same results as bare array format
out_array=$(bash "$DECISION" < "$FIXTURES/decision/mixed.json")
out_object=$(bash "$DECISION" < "$FIXTURES/decision/mixed-object.json")
assert_output "decision: object format matches array format output" "$out_array" "$out_object"

# --- decide-summary.sh accepts object format ---

summary=$(bash "$SCRIPT_DIR/decide-summary.sh" --targets '[]' "$FIXTURES/decision/mixed-object.json")
assert_contains "decide-summary: object format detects branch" "$summary" "oadp-dev"
assert_contains "decide-summary: object format lists skipped repos" "$summary" "skip=true"

# --- jq merge pattern works with object format ---

merged=$(echo '[]' "$(cat "$FIXTURES/decision/mixed-object.json")" | jq -s '[ .[] | if type == "object" then .repos else . end ] | add')
count=$(echo "$merged" | jq 'length')
assert_output "jq merge: object format produces correct count" "8" "$count"

# --- rebase-status-notify.sh accepts object format via stdin ---

# Wrap a minimal status array in object format
minimal_status='{"velero_tag_alignment": {"tag": "v1.14.0", "tag_sha": "abc123", "all_aligned": true, "repos": []}, "repos": [{"repo": "openshift/velero", "branch": "oadp-1.4", "wave": 2, "skip": false, "upstream": "", "checks": {"config": {"status": "ok", "summary": ""}, "open_pr": {"status": "na", "summary": ""}, "dep_sync": {"status": "ok", "summary": ""}}}]}'
notify_out=$(echo "$minimal_status" | bash "$NOTIFY_STATUS")
if echo "$notify_out" | jq -e '.empty // false' >/dev/null 2>&1 || echo "$notify_out" | jq -e '.blocks' >/dev/null 2>&1; then
    printf "  PASS  status-notify: object format produces valid payload\n"
    passed=$((passed + 1))
else
    printf "  FAIL  status-notify: object format produces valid payload\n"
    printf "    output: %s\n" "$(echo "$notify_out" | head -3)"
    failed=$((failed + 1))
fi

# Bare array should also still work
bare_status='[{"repo": "openshift/velero", "branch": "oadp-1.4", "wave": 2, "skip": false, "upstream": "", "checks": {"config": {"status": "ok", "summary": ""}, "open_pr": {"status": "na", "summary": ""}, "dep_sync": {"status": "ok", "summary": ""}}}]'
notify_bare=$(echo "$bare_status" | bash "$NOTIFY_STATUS")
if echo "$notify_bare" | jq -e '.empty // false' >/dev/null 2>&1 || echo "$notify_bare" | jq -e '.blocks' >/dev/null 2>&1; then
    printf "  PASS  status-notify: bare array format still works\n"
    passed=$((passed + 1))
else
    printf "  FAIL  status-notify: bare array format still works\n"
    printf "    output: %s\n" "$(echo "$notify_bare" | head -3)"
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
