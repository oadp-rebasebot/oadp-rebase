#!/bin/bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────
# verify-rebase.sh — Deterministic verification that downstream
# carry commits and files survive a rebase.
#
# Usage:
#   verify-rebase.sh <dest-repo> <dest-branch> \
#                    <rebase-repo> <rebase-branch> \
#                    <upstream-repo> <upstream-ref>
#
# Exit codes: 0 = pass, 1 = verification failure, 2 = usage error
# ──────────────────────────────────────────────────────────────

log_info()    { printf "ℹ️  %s\n" "$*"; }
log_success() { printf "✅ %s\n" "$*"; }
log_fail()    { printf "❌ %s\n" "$*"; }
log_warn()    { printf "⚠️  %s\n" "$*"; }

usage() {
  cat <<'EOF'
Usage: verify-rebase.sh <dest-repo> <dest-branch> \
                        <rebase-repo> <rebase-branch> \
                        <upstream-repo> <upstream-ref>

Arguments:
  dest-repo       Downstream repo (e.g. openshift/velero)
  dest-branch     Downstream branch (e.g. oadp-1.6)
  rebase-repo     Rebase working repo (e.g. oadp-rebasebot/velero)
  rebase-branch   Rebase branch (e.g. rebase-bot-oadp-1.6)
  upstream-repo   Upstream repo (e.g. velero-io/velero)
  upstream-ref    Upstream tag or branch (e.g. v1.18.2-rc.2)

Exit codes:
  0  All checks passed
  1  Verification failed (missing commits or files)
  2  Usage / input error
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -ne 6 ]]; then
  usage >&2
  exit 2
fi

DEST_REPO="$1"
DEST_BRANCH="$2"
REBASE_REPO="$3"
REBASE_BRANCH="$4"
UPSTREAM_REPO="$5"
UPSTREAM_REF="$6"

OVERALL=0  # 0 = pass, set to 1 on any failure
SUMMARY_LINES=()

# ── Phase 1: Setup ───────────────────────────────────────────

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

DEST_CARRIES="$WORK_DIR/dest_carries.txt"
REBASE_CARRIES="$WORK_DIR/rebase_carries.txt"
MISSING_CARRIES="$WORK_DIR/missing_carries.txt"
DEST_FILES="$WORK_DIR/dest_files.txt"
UPSTREAM_FILES="$WORK_DIR/upstream_files.txt"
REBASE_FILES="$WORK_DIR/rebase_files.txt"
DOWNSTREAM_ONLY="$WORK_DIR/downstream_only.txt"
MISSING_FILES="$WORK_DIR/missing_files.txt"

log_info "Verifying rebase: ${DEST_REPO}:${DEST_BRANCH} → ${REBASE_REPO}:${REBASE_BRANCH}"
log_info "Upstream: ${UPSTREAM_REPO}:${UPSTREAM_REF}"

BARE_REPO="$WORK_DIR/repo.git"
git init --bare "$BARE_REPO" -q
cd "$BARE_REPO"

git remote add dest "https://github.com/${DEST_REPO}.git"
git remote add rebase "https://github.com/${REBASE_REPO}.git"
git remote add upstream "https://github.com/${UPSTREAM_REPO}.git"

log_info "Fetching dest branch..."
if ! git fetch dest "$DEST_BRANCH" -q 2>/dev/null; then
  log_fail "Could not fetch ${DEST_REPO}:${DEST_BRANCH}"
  exit 2
fi

log_info "Fetching rebase branch..."
if ! git fetch rebase "$REBASE_BRANCH" -q 2>/dev/null; then
  log_fail "Could not fetch ${REBASE_REPO}:${REBASE_BRANCH}"
  exit 2
fi

log_info "Fetching upstream ref..."
if ! git fetch upstream "$UPSTREAM_REF" --depth=1 -q 2>/dev/null; then
  log_fail "Could not fetch ${UPSTREAM_REPO}:${UPSTREAM_REF}"
  exit 2
fi

DEST_REF="refs/remotes/dest/${DEST_BRANCH}"
REBASE_REF="refs/remotes/rebase/${REBASE_BRANCH}"
UPSTREAM_FETCH="FETCH_HEAD"

# Resolve refs
DEST_SHA="$(git rev-parse "$DEST_REF")"
REBASE_SHA="$(git rev-parse "$REBASE_REF")"
UPSTREAM_SHA="$(git rev-parse "$UPSTREAM_FETCH")"

# ── Phase 2: Carry Commit Verification ──────────────────────

log_info "Checking carry commits..."

git log --format='%s' --grep='UPSTREAM: <carry>' "$DEST_SHA" | sort -u > "$DEST_CARRIES"
git log --format='%s' --grep='UPSTREAM: <carry>' "$REBASE_SHA" | sort -u > "$REBASE_CARRIES"

EXPECTED_COUNT=$(wc -l < "$DEST_CARRIES" | tr -d ' ')
ACTUAL_COUNT=$(wc -l < "$REBASE_CARRIES" | tr -d ' ')

comm -23 "$DEST_CARRIES" "$REBASE_CARRIES" > "$MISSING_CARRIES"
MISSING_COUNT=$(wc -l < "$MISSING_CARRIES" | tr -d ' ')

if [[ "$MISSING_COUNT" -gt 0 ]]; then
  log_fail "Carry commits: ${MISSING_COUNT} MISSING (expected ${EXPECTED_COUNT}, found ${ACTUAL_COUNT})"
  printf "    Missing commits:\n"
  while IFS= read -r line; do
    printf "      - %s\n" "$line"
    if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
      printf "::error::Missing carry commit: %s\n" "$line"
    fi
  done < "$MISSING_CARRIES"
  OVERALL=1
  SUMMARY_LINES+=("| Carry commits | FAIL | ${ACTUAL_COUNT}/${EXPECTED_COUNT} found, **${MISSING_COUNT} missing** |")
else
  log_success "Carry commits: ${EXPECTED_COUNT}/${EXPECTED_COUNT} preserved"
  SUMMARY_LINES+=("| Carry commits | PASS | ${EXPECTED_COUNT}/${EXPECTED_COUNT} found |")
fi

# ── Phase 3: Downstream-Only File Verification ──────────────

log_info "Checking downstream-only files..."

git ls-tree -r --name-only "$DEST_SHA" | sort > "$DEST_FILES"
git ls-tree -r --name-only "$UPSTREAM_SHA" | sort > "$UPSTREAM_FILES"
git ls-tree -r --name-only "$REBASE_SHA" | sort > "$REBASE_FILES"

comm -23 "$DEST_FILES" "$UPSTREAM_FILES" > "$DOWNSTREAM_ONLY"
DS_FILE_COUNT=$(wc -l < "$DOWNSTREAM_ONLY" | tr -d ' ')

comm -23 "$DOWNSTREAM_ONLY" "$REBASE_FILES" > "$MISSING_FILES"
MISSING_FILE_COUNT=$(wc -l < "$MISSING_FILES" | tr -d ' ')

if [[ "$MISSING_FILE_COUNT" -gt 0 ]]; then
  log_fail "Downstream files: ${MISSING_FILE_COUNT} MISSING out of ${DS_FILE_COUNT} downstream-only files"
  printf "    Missing files:\n"
  while IFS= read -r line; do
    printf "      - %s\n" "$line"
    if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
      printf "::error::Missing downstream file: %s\n" "$line"
    fi
  done < "$MISSING_FILES"
  OVERALL=1
  FOUND_FILES=$((DS_FILE_COUNT - MISSING_FILE_COUNT))
  SUMMARY_LINES+=("| Downstream files | FAIL | ${FOUND_FILES}/${DS_FILE_COUNT} found, **${MISSING_FILE_COUNT} missing** |")
else
  log_success "Downstream files: ${DS_FILE_COUNT}/${DS_FILE_COUNT} preserved"
  SUMMARY_LINES+=("| Downstream files | PASS | ${DS_FILE_COUNT}/${DS_FILE_COUNT} found |")
fi

# ── Phase 4: Hook Validation ────────────────────────────────

DROP_COUNT=$(git log --format='%s' --grep='UPSTREAM: <drop>' "$REBASE_SHA" | wc -l | tr -d ' ')

if [[ "$DROP_COUNT" -eq 0 ]]; then
  log_warn "Hook commits: 0 UPSTREAM: <drop> commits found (hooks may not have run)"
  SUMMARY_LINES+=("| Hook commits | WARN | 0 found |")
else
  log_success "Hook commits: ${DROP_COUNT} UPSTREAM: <drop> commits found"
  SUMMARY_LINES+=("| Hook commits | PASS | ${DROP_COUNT} found |")
fi

# ── Phase 5: Report ─────────────────────────────────────────

printf "\n"
printf "═══════════════════════════════════════════════\n"
if [[ "$OVERALL" -eq 0 ]]; then
  log_success "REBASE VERIFIED"
else
  log_fail "REBASE VERIFICATION FAILED"
fi
printf "═══════════════════════════════════════════════\n"

write_markdown_report() {
  local dest="$1"
  {
    if [[ "$OVERALL" -eq 0 ]]; then
      printf "## :white_check_mark: Rebase Verification Passed\n\n"
    else
      printf "## :x: Rebase Verification Failed\n\n"
    fi
    printf "**Dest:** \`%s:%s\`  \n" "$DEST_REPO" "$DEST_BRANCH"
    printf "**Rebase:** \`%s:%s\`  \n" "$REBASE_REPO" "$REBASE_BRANCH"
    printf "**Upstream:** \`%s:%s\`\n\n" "$UPSTREAM_REPO" "$UPSTREAM_REF"
    printf "| Check | Result | Details |\n"
    printf "|-------|--------|---------|\n"
    for line in "${SUMMARY_LINES[@]}"; do
      printf "%s\n" "$line"
    done

    if [[ "$MISSING_COUNT" -gt 0 ]]; then
      printf "\n### Missing Carry Commits\n\n"
      while IFS= read -r line; do
        printf "- \`%s\`\n" "$line"
      done < "$MISSING_CARRIES"
    fi

    if [[ "$MISSING_FILE_COUNT" -gt 0 ]]; then
      printf "\n### Missing Downstream Files\n\n"
      while IFS= read -r line; do
        printf "- \`%s\`\n" "$line"
      done < "$MISSING_FILES"
    fi
  } >> "$dest"
}

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  write_markdown_report "$GITHUB_STEP_SUMMARY"
fi

if [[ -n "${COMMENT_FILE:-}" ]]; then
  write_markdown_report "$COMMENT_FILE"
fi

exit "$OVERALL"
