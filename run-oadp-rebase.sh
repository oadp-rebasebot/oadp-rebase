#!/bin/sh

set -eu

#
# OADP Rebase Runner Script
# Unified interface for running rebase operations for single OADP repos or entire waves
#

# === Configuration ===

GITHUB_APP_ID="${GITHUB_APP_ID:-1810299}"
GITHUB_CLONER_ID="${GITHUB_CLONER_ID:-1810272}"
GIT_USERNAME="${GIT_USERNAME:-oadp-team-rebase-bot}"
GIT_EMAIL="${GIT_EMAIL:-oadp-maintainers@redhat.com}"
# In the SECRETS_DIR, there are needed two private keys for the GITHUB APPs
# oadp-rebasebot-app-key
# oadp-rebasebot-cloner-key
SECRETS_DIR="${SECRETS_DIR:-${HOME}/.rebasebot/secrets}"
REBASEBOT_IMAGE="${REBASEBOT_IMAGE:-quay.io/migtools/rebasebot:latest}"
OADP_BRANCH="${OADP_BRANCH:-oadp-dev}"
OADP_BRANCH_SET=""

# === Repository Configuration Mapping ===

get_config_name() {
    case "$1" in
        # === Udistribution ===
        udistribution-main) echo "migtools_udistribution_main" ;;

        # === Wave 1 ===
        kopia-oadp-dev) echo "migtools_kopia_oadp-dev" ;;
        kopia-oadp-1.5) echo "migtools_kopia_oadp-1.5" ;;
        restic-oadp-dev) echo "openshift_restic_oadp-dev" ;;
        restic-oadp-1.5) echo "openshift_restic_oadp-1.5" ;;

        # === Wave 2 ===
        velero-oadp-dev) echo "openshift_velero_oadp-dev" ;;
        velero-oadp-1.5) echo "openshift_velero_oadp-1.5" ;;

        # === Wave 3 ===
        velero-plugin-for-csi-oadp-dev) echo "openshift_velero_plugin_for_csi_oadp-dev" ;;
        oadp-operator-oadp-dev) echo "openshift_oadp-operator_oadp-dev" ;;
        oadp-operator-oadp-1.5) echo "openshift_oadp-operator_oadp-1.5" ;;
        velero-plugin-for-aws-oadp-dev) echo "openshift_velero_plugin_for_aws_oadp-dev" ;;
        velero-plugin-for-aws-oadp-1.5) echo "openshift_velero_plugin_for_aws_oadp-1.5" ;;
        velero-plugin-for-legacy-aws-oadp-dev) echo "openshift_velero_plugin_for_legacy_aws_oadp-dev" ;;
        velero-plugin-for-legacy-aws-oadp-1.5) echo "openshift_velero_plugin_for_legacy_aws_oadp-1.5" ;;
        velero-plugin-for-microsoft-azure-oadp-dev) echo "openshift_velero_plugin_for_microsoft_azure_oadp-dev" ;;
        velero-plugin-for-microsoft-azure-oadp-1.5) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.5" ;;

        # === Wave 4 ===
        oadp-non-admin-oadp-dev) echo "migtools_oadp_non_admin_oadp-dev" ;;
        oadp-non-admin-oadp-1.5) echo "migtools_oadp_non_admin_oadp-1.5" ;;
        openshift-velero-plugin-oadp-dev) echo "openshift_openshift_velero_plugin_oadp-dev" ;;
        openshift-velero-plugin-oadp-1.5) echo "openshift_openshift_velero_plugin_oadp-1.5" ;;

        # === Wave 5 ===
        oadp-must-gather-oadp-dev) echo "openshift_oadp_must_gather_oadp-dev" ;;
        oadp-must-gather-oadp-1.5) echo "openshift_oadp_must_gather_oadp-1.5" ;;

        # === OADP CLI ===
        oadp-cli-oadp-dev) echo "migtools_oadp_cli_oadp-dev" ;;

        # === Unknown ===
        *) return 1 ;;
    esac
}

# === Wave Configuration ===

get_wave_repos() {
    branch="$1"
    wave="$2"

    # Special case for udistribution: always include main in wave 1
    if [ "$wave" -eq 1 ]; then
        if [ "$branch" = "oadp-dev" ] || [ "$branch" = "main" ]; then
            echo "udistribution-main kopia-oadp-dev restic-oadp-dev"
            return 0
        fi
    fi

    if [ "$branch" = "oadp-dev" ]; then
        case "$wave" in
            2) echo "velero-oadp-dev" ;;
            3) echo "velero-plugin-for-csi-oadp-dev oadp-operator-oadp-dev velero-plugin-for-aws-oadp-dev velero-plugin-for-legacy-aws-oadp-dev velero-plugin-for-microsoft-azure-oadp-dev" ;;
            4) echo "oadp-non-admin-oadp-dev openshift-velero-plugin-oadp-dev" ;;
            5) echo "oadp-must-gather-oadp-dev oadp-cli-oadp-dev" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.5" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.5 restic-oadp-1.5" ;;
            2) echo "velero-oadp-1.5" ;;
            3) echo "oadp-operator-oadp-1.5 velero-plugin-for-aws-oadp-1.5 velero-plugin-for-legacy-aws-oadp-1.5 velero-plugin-for-microsoft-azure-oadp-1.5" ;;
            4) echo "oadp-non-admin-oadp-1.5 openshift-velero-plugin-oadp-1.5" ;;
            5) echo "oadp-must-gather-oadp-1.5" ;;
            *) return 1 ;;
        esac
    else
        return 1
    fi
}

# === Utility Functions ===

error_exit() { printf "❌  %s\n" "$*" >&2; exit 1; }
log_section() { printf "\n==========================================\n%s\n==========================================\n" "$*"; }
log_info() { printf "ℹ️  %s\n" "$*"; }
log_success() { printf "✅ %s\n" "$*"; }
log_fail() { printf "❌  %s\n" "$*"; }
log_warn() { printf "⚠️  %s\n" "$*"; }

usage() {
    cat <<EOF
OADP Rebase Runner

Usage: $0 [OPTIONS] <target>

Arguments:
  target    Repository (exact repo-branch) or wave number

Options:
  -d, --dry-run          Dry-run mode
  -t, --test             Test configuration only (local only)
  -b, --branch BRANCH    Specify branch (default: $OADP_BRANCH)
  -w, --wave             Execute entire wave
  -s, --secrets-dir DIR  Secrets directory
  -r, --remote           Use remote configuration
  -h, --help             Show this help
EOF
}

check_secrets() {
    [ -d "$SECRETS_DIR" ] || error_exit "Secrets directory $SECRETS_DIR not found"
    [ -f "$SECRETS_DIR/oadp-rebasebot-app-key" ] || error_exit "Missing app key: $SECRETS_DIR/oadp-rebasebot-app-key"
    [ -f "$SECRETS_DIR/oadp-rebasebot-cloner-key" ] || error_exit "Missing cloner key: $SECRETS_DIR/oadp-rebasebot-cloner-key"
    log_info "Using app key: $SECRETS_DIR/oadp-rebasebot-app-key"
    log_info "Using cloner key: $SECRETS_DIR/oadp-rebasebot-cloner-key"
}

load_config() {
    config="$1"
    source_type="$2"
    config_name="$(get_config_name "$config")" || error_exit "Unknown config '$config'"
    config_file="${config_name}.env.sh"

    log_info "Loading ${source_type} configuration: ${config_file}"

    # Required to not left some variables from the previous run that may have been
    # set for different config
    [ -n "${SOURCE_UPSTREAM_REPO:-}" ] && unset SOURCE_UPSTREAM_REPO
    [ -n "${DESTINATION_DOWNSTREAM_REPO:-}" ] && unset DESTINATION_DOWNSTREAM_REPO
    [ -n "${REBASE_REPO:-}" ] && unset REBASE_REPO
    [ -n "${HOOK_SCRIPTS:-}" ] && unset HOOK_SCRIPTS
    [ -n "${EXTRA_REBASEBOT_ARGS:-}" ] && unset EXTRA_REBASEBOT_ARGS
    [ -n "${SKIP_REPO:-}" ] && unset SKIP_REPO

    if [ "$source_type" = "local" ]; then
        [ -f "rebase-configs/$config_file" ] || error_exit "Config file not found: rebase-configs/${config_file}"
        . "rebase-configs/$config_file"
    else
        config_url="https://raw.githubusercontent.com/oadp-rebasebot/oadp-rebase/refs/heads/oadp-dev/rebase-configs/$config_file"
        temp_config="$(mktemp)"
        trap 'rm -f "$temp_config"' EXIT
        curl --fail --silent --show-error "$config_url" > "$temp_config" || error_exit "Failed to load remote config: ${config_url}"
        . "$temp_config"
        trap - EXIT
        rm -f "$temp_config"
    fi

    log_success "Config loaded"
}

test_config() {
    config="$1"
    log_info "Testing local config: $config"
    load_config "$config" "local"
    # Check if this repo should be skipped
    if [ "${SKIP_REPO:-false}" = "true" ]; then
        log_warn "Skipping $config (SKIP_REPO=true in config)"
        return 0
    fi
    log_info "SOURCE_UPSTREAM_REPO: ${SOURCE_UPSTREAM_REPO:-<not set>}"
    log_info "DESTINATION_DOWNSTREAM_REPO: ${DESTINATION_DOWNSTREAM_REPO:-<not set>}"
    log_info "REBASE_REPO: ${REBASE_REPO:-<not set>}"
    [ -n "${HOOK_SCRIPTS:-}" ] && log_info "HOOK_SCRIPTS: $HOOK_SCRIPTS"
    [ -n "${EXTRA_REBASEBOT_ARGS:-}" ] && log_info "EXTRA_REBASEBOT_ARGS: $EXTRA_REBASEBOT_ARGS"
}

run_container_rebase() {
    config="$1"
    dry_run="$2"
    source_type="$3"

    CONTAINER_ENGINE="$(command -v docker || true)"
    [ -z "$CONTAINER_ENGINE" ] && CONTAINER_ENGINE="$(command -v podman || true)"
    [ -z "$CONTAINER_ENGINE" ] && error_exit "No docker or podman found"

    log_section "Rebasing using recepit: $config"
    log_info "Dry run: $dry_run, Config source: $source_type"

    check_secrets
    load_config "$config" "$source_type"
    # Check if this repo should be skipped
    if [ "${SKIP_REPO:-false}" = "true" ]; then
        log_warn "Skipping $config (SKIP_REPO=true in config)"
        return 0
    fi

    CMD="$CONTAINER_ENGINE run --rm --pull=always \
  -v \"$SECRETS_DIR:/secrets:Z,ro\" \
  -e GIT_USERNAME=\"$GIT_USERNAME\" \
  -e GIT_EMAIL=\"$GIT_EMAIL\" \
  \"$REBASEBOT_IMAGE\" \
  --source \"$SOURCE_UPSTREAM_REPO\" \
  --dest \"$DESTINATION_DOWNSTREAM_REPO\" \
  --rebase \"$REBASE_REPO\" \
  --git-username \"$GIT_USERNAME\" \
  --git-email \"$GIT_EMAIL\" \
  --github-app-id \"$GITHUB_APP_ID\" \
  --github-app-key /secrets/oadp-rebasebot-app-key \
  --github-cloner-id \"$GITHUB_CLONER_ID\" \
  --github-cloner-key /secrets/oadp-rebasebot-cloner-key"

    [ -n "${HOOK_SCRIPTS:-}" ] && CMD="$CMD $HOOK_SCRIPTS"
    [ -n "${EXTRA_REBASEBOT_ARGS:-}" ] && CMD="$CMD $EXTRA_REBASEBOT_ARGS"
    [ "$dry_run" = "true" ] && CMD="$CMD --dry-run"

    log_info "Command:"
    log_info "\$ ${CMD}"
    sh -c "$CMD"
}

run_wave() {
    wave_num="$1"
    dry_run="$2"
    source_type="$3"

    repos="$(get_wave_repos "$OADP_BRANCH" "$wave_num")" || error_exit "Unknown wave '$wave_num' for branch $OADP_BRANCH"
    log_info "Wave $wave_num repositories: $repos"

    # === Pre-check: ensure all configs exist ===
    missing_configs=""
    for config in $repos; do
        config_name="$(get_config_name "$config")" || { missing_configs="$missing_configs $config"; continue; }
        config_file="rebase-configs/${config_name}.env.sh"
        if [ ! -f "$config_file" ]; then
            missing_configs="$missing_configs $config_file"
        fi
    done

    if [ -n "$missing_configs" ]; then
        log_fail "Aborting wave $wave_num: the following config(s) are missing:"
        for repo in $missing_configs; do
            [ -n "$repo" ] && printf "  %s\n" "$repo"
        done
        return 1
    fi

    # === Wave execution ===
    failed=""
    skipped=""
    success_count=0

    for config in $repos; do
        log_section "Processing repo: $config"

        # Check if config exists
        if ! get_config_name "$config" >/dev/null 2>&1; then
            log_warn "Skipping $config (no config)"
            skipped="$skipped $config"
            continue
        fi

        # Load config to check SKIP_REPO flag
        load_config "$config" "$source_type"
        if [ "${SKIP_REPO:-false}" = "true" ]; then
            log_warn "Skipping $config (SKIP_REPO=true in config)"
            skipped="$skipped $config"
            continue
        fi

        # Run rebase
        if run_container_rebase "$config" "$dry_run" "$source_type"; then
            log_success "Processed $config"
            success_count=$((success_count + 1))
        else
            log_fail "Failed $config"
            failed="$failed $config"
        fi
    done

    total_count=$(echo "$repos" | wc -w)
    failed_count=$(echo "$failed" | wc -w)
    skipped_count=$(echo "$skipped" | wc -w)

    # Always print summary
    log_section "Wave $wave_num summary"
    log_info "Total repos in wave $wave_num: $total_count"
    log_info "✅ Success: $success_count"
    log_info "❌ Failed: $failed_count"
    log_info "⚠️ Skipped: $skipped_count"

    if [ -n "$skipped" ]; then
        log_info "Skipped repositories:"
        for repo in $skipped; do [ -n "$repo" ] && printf "  %s\n" "$repo"; done
    fi

    if [ -n "$failed" ]; then
        log_fail "Failed repositories:"
        for repo in $failed; do [ -n "$repo" ] && printf "  %s\n" "$repo"; done
        return 1
    fi

    log_success "All repositories processed (or skipped) successfully!"
}

# === Argument Parsing ===

DRY_RUN="false"
TEST_MODE="false"
WAVE_MODE="false"
REMOTE_MODE="false"
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        -d|--dry-run) DRY_RUN="true"; shift ;;
        -t|--test) TEST_MODE="true"; shift ;;
        -w|--wave) WAVE_MODE="true"; shift ;;
        -r|--remote) REMOTE_MODE="true"; shift ;;
        -b|--branch) OADP_BRANCH="$2"; OADP_BRANCH_SET=1; shift 2 ;;
        -s|--secrets-dir) SECRETS_DIR="$2"; shift 2 ;;
        -*) error_exit "Unknown option: $1" ;;
        *) [ -z "$TARGET" ] || error_exit "Multiple targets specified"; TARGET="$1"; shift ;;
    esac
done

[ -n "$TARGET" ] || { usage; error_exit "Target is required"; }

SOURCE_TYPE="local"
[ "$REMOTE_MODE" = "true" ] && SOURCE_TYPE="remote"

# Special handling for udistribution default branch
if [ "$WAVE_MODE" != "true" ]; then
    if [ "$TARGET" = "udistribution" ] && [ -z "${OADP_BRANCH_SET:-}" ]; then
        TARGET="udistribution-main"
    elif ! get_config_name "$TARGET" >/dev/null 2>&1; then
        TARGET="${TARGET}-${OADP_BRANCH}"
    fi
fi

if [ "$WAVE_MODE" = "true" ]; then
    [ "$TEST_MODE" = "true" ] && error_exit "Test mode not supported for wave"
    run_wave "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
else
    get_config_name "$TARGET" >/dev/null || error_exit "Unknown config '$TARGET'"
    if [ "$TEST_MODE" = "true" ]; then
        test_config "$TARGET"
    else
        run_container_rebase "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
    fi
fi

