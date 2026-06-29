#!/bin/bash
set -euo pipefail

# Orchestrates: dry-run → triage → retry → verify → real-run
# Outputs a JSON result file and sets exit code.
#
# Usage: run-rebase-pipeline.sh <target> [--dry-run-only] [--secrets-dir DIR]
#
# Requires: run-oadp-rebase.sh, tools/auto-rebase/resolve-config.sh,
#           tools/auto-rebase/conflict-triage.sh, tools/verify-rebase.sh

SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$SCRIPT_DIR"

TARGET=""
DRY_RUN_ONLY=false
SECRETS_DIR="${SECRETS_DIR:-/tmp/rebasebot-secrets}"
RESULTS_DIR="${RESULTS_DIR:-/tmp/results}"
WORK_DIR="/tmp/rebase-workdir"
WORK_DIR_RETRY="/tmp/rebase-workdir-retry"

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run-only) DRY_RUN_ONLY=true; shift ;;
        --secrets-dir) SECRETS_DIR="$2"; shift 2 ;;
        --results-dir) RESULTS_DIR="$2"; shift 2 ;;
        -h|--help)
            echo "Usage: run-rebase-pipeline.sh <target> [--dry-run-only] [--secrets-dir DIR] [--results-dir DIR]"
            exit 0
            ;;
        *) [ -z "$TARGET" ] || { echo "Multiple targets specified" >&2; exit 2; }; TARGET="$1"; shift ;;
    esac
done

[ -z "$TARGET" ] && { echo "Usage: run-rebase-pipeline.sh <target>" >&2; exit 2; }

log_info()  { printf "ℹ️  %s\n" "$*"; }
log_ok()    { printf "✅ %s\n" "$*"; }
log_fail()  { printf "❌ %s\n" "$*"; }
end_phase() { [ "${_IN_PHASE:-}" = "true" ] && echo "::endgroup::" || true; _IN_PHASE=false; }
log_phase() { end_phase; echo "::group::$*"; _IN_PHASE=true; }
trap end_phase EXIT

# Resolve config
eval "$(bash tools/auto-rebase/resolve-config.sh "$TARGET")"

# Result tracking
dry_run_rc=""
dry_run_policy="strict"
triage_verdict=""
verify_dry_run_rc=""
real_run_rc=""
pr_url=""
pr_info=""

parse_pr_output() {
    local output_file="$1"
    pr_url=$(grep -oE 'https://github.com/[^[:space:]]+/pull/[0-9]+' "$output_file" | head -1 || echo "")
    pr_info=$(grep -E '(I created a new rebase PR|I updated existing rebase PR|PR .+/pull/[0-9]+ already contains|rebase/manual)' \
        "$output_file" | sed 's/.*INFO - //' | head -1 || echo "")
}

# ── Phase 1: Dry-run (strict) ──────────────────────────────
log_phase "Phase 1: Dry-run (strict)"

set +e
./run-oadp-rebase.sh --dry-run --working-dir "$WORK_DIR" "$TARGET" 2>&1 | tee /tmp/rebase-output.txt
dry_run_rc=${PIPESTATUS[0]}
set -e

if [ "$dry_run_rc" = "0" ]; then
    log_ok "Dry-run strict succeeded"
else
    log_fail "Dry-run strict failed (rc=${dry_run_rc})"

    # ── Triage ──────────────────────────────────────────────
    log_phase "Triage"

    set +e
    triage_verdict=$(bash tools/auto-rebase/conflict-triage.sh "rebase-configs/${CONFIG_NAME}.env.sh" < /tmp/rebase-output.txt)
    set -e

    triage_safe=$(echo "$triage_verdict" | jq -r '.safe // false')
    triage_reason=$(echo "$triage_verdict" | jq -r '.reason // "unknown"')
    log_info "Triage: safe=${triage_safe} — ${triage_reason}"

    if [ "$triage_safe" = "true" ]; then
        # ── Retry with warn ─────────────────────────────────
        log_phase "Phase 1b: Dry-run (warn)"

        set +e
        ./run-oadp-rebase.sh --dry-run --working-dir "$WORK_DIR_RETRY" --conflict-policy warn "$TARGET" 2>&1 | tee /tmp/rebase-output-retry.txt
        dry_run_rc=${PIPESTATUS[0]}
        set -e

        if [ "$dry_run_rc" = "0" ]; then
            dry_run_policy="warn"
            WORK_DIR="$WORK_DIR_RETRY"
            log_ok "Dry-run warn succeeded"
        else
            log_fail "Dry-run warn also failed (rc=${dry_run_rc})"
        fi
    fi
fi

# ── Phase 2: Verify dry-run locally ────────────────────────
if [ "$dry_run_rc" = "0" ]; then
    log_phase "Phase 2: Verify dry-run"

    local_repo="${WORK_DIR}/${REPO_NAME}"

    set +e
    bash tools/verify-rebase.sh \
        "$DEST_REPO" "$DEST_BRANCH" \
        "$local_repo" "rebase" \
        "$UPSTREAM_REPO" "$UPSTREAM_REF"
    verify_dry_run_rc=$?
    set -e

    if [ "$verify_dry_run_rc" = "0" ]; then
        log_ok "Dry-run verification passed"
    else
        log_fail "Dry-run verification failed (rc=${verify_dry_run_rc})"
    fi
fi

# ── Phase 3: Real run ──────────────────────────────────────
if [ "$DRY_RUN_ONLY" = "true" ]; then
    log_info "Dry-run only mode — skipping real run"
elif [ "${verify_dry_run_rc:-1}" != "0" ]; then
    log_fail "Skipping real run — dry-run verification failed"
else
    log_phase "Phase 3: Real run"

    conflict_flag=""
    [ "$dry_run_policy" = "warn" ] && conflict_flag="--conflict-policy warn"

    set +e
    ./run-oadp-rebase.sh $conflict_flag "$TARGET" 2>&1 | tee /tmp/rebase-output-real.txt
    real_run_rc=${PIPESTATUS[0]}
    set -e

    parse_pr_output /tmp/rebase-output-real.txt

    if [ "$real_run_rc" = "0" ]; then
        log_ok "Real run succeeded"
    else
        log_fail "Real run failed (rc=${real_run_rc})"
    fi
fi

log_phase "Write result"
mkdir -p "$RESULTS_DIR"

# Determine effective exit code
if [ -n "$real_run_rc" ]; then
    exit_code="$real_run_rc"
else
    exit_code="${dry_run_rc}"
fi

# Determine verify result
if [ "${verify_dry_run_rc:-}" = "0" ]; then
    verify_result="pass"
elif [ -n "${verify_dry_run_rc:-}" ]; then
    verify_result="fail"
else
    verify_result="skipped"
fi

# Check for manual intervention
is_manual=false
grep -qE 'rebase/manual' /tmp/rebase-output.txt /tmp/rebase-output-retry.txt /tmp/rebase-output-real.txt 2>/dev/null && is_manual=true

jq -n \
    --arg target "$TARGET" \
    --arg exit_code "$exit_code" \
    --arg pr_url "${pr_url:-}" \
    --arg pr_info "${pr_info:-}" \
    --arg verify "$verify_result" \
    --arg dry_run_rc "$dry_run_rc" \
    --arg dry_run_policy "$dry_run_policy" \
    --arg verify_dry_run_rc "${verify_dry_run_rc:-}" \
    --arg real_run_rc "${real_run_rc:-}" \
    --argjson is_manual "$is_manual" \
    --argjson dry_run_only "$DRY_RUN_ONLY" \
    '{target: $target, exit_code: $exit_code, pr_url: $pr_url, pr_info: $pr_info,
      verify: $verify, dry_run_rc: $dry_run_rc, dry_run_policy: $dry_run_policy,
      verify_dry_run_rc: $verify_dry_run_rc, real_run_rc: $real_run_rc,
      is_manual: $is_manual, dry_run_only: $dry_run_only}' \
    > "${RESULTS_DIR}/${TARGET}.json"

log_info "Result written to ${RESULTS_DIR}/${TARGET}.json"

# Exit with appropriate code
if [ "$is_manual" = "true" ]; then
    exit 0
fi
exit "${exit_code}"
