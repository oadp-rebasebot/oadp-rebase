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

# === Repository Configuration Mapping ===

get_config_name() {
    # Returns the configuration file base name corresponding to a repository.
    #
    # Each repository has a config file in rebase-configs/ named as:
    #   <CONFIG_NAME>.env.sh
    #
    # Usage example:
    #   udistribution-main) echo "migtools_udistribution_main" ;;
    #
    # Input: repository name (e.g., udistribution-main)
    # Output: config name (e.g., migtools_udistribution_main)
    #
    case "$1" in
        # === Udistribution ===
        udistribution-main) echo "migtools_udistribution_main" ;;

        # === Wave 1 ===
        # ==============

        # === Kopia ===
        kopia-oadp-dev) echo "migtools_kopia_oadp-dev" ;;
        kopia-oadp-1.5) echo "migtools_kopia_oadp-1.5" ;;

        # === Restic ===
        restic-oadp-dev) echo "openshift_restic_oadp-dev" ;;
        restic-oadp-1.5) echo "openshift_restic_oadp-1.5" ;;

        # === Wave 2 ===
        # ==============

        # === Velero Core ===
        velero-oadp-dev) echo "openshift_velero_oadp-dev" ;;
        velero-oadp-1.5) echo "openshift_velero_oadp-1.5" ;;

        # === Wave 3 ===
        # ==============

        # === Velero CSI Plugin ===
        velero-plugin-for-csi-oadp-dev) echo "openshift_velero_plugin_for_csi_oadp-dev" ;;
        velero-plugin-for-csi-oadp-1.5) echo "openshift_velero_plugin_for_csi_oadp-1.5" ;;

        # === OADP Operator ===
        oadp-operator-oadp-dev) echo "openshift_oadp_operator_oadp-dev" ;;
        oadp-operator-oadp-1.5) echo "openshift_oadp_operator_oadp-1.5" ;;

        # === Velero AWS Plugin ===
        velero-plugin-for-aws-oadp-dev) echo "openshift_velero_plugin_for_aws_oadp-dev" ;;
        velero-plugin-for-aws-oadp-1.5) echo "openshift_velero_plugin_for_aws_oadp-1.5" ;;

        # === Velero Legacy AWS Plugin ===
        velero-plugin-for-legacy-aws-oadp-dev) echo "openshift_velero_plugin_for_legacy_aws_oadp-dev" ;;
        velero-plugin-for-legacy-aws-oadp-1.5) echo "openshift_velero_plugin_for_legacy_aws_oadp-1.5" ;;

        # === Velero Azure Plugin ===
        velero-plugin-for-microsoft-azure-oadp-dev) echo "openshift_velero_plugin_for_microsoft_azure_oadp-dev" ;;
        velero-plugin-for-microsoft-azure-oadp-1.5) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.5" ;;

        # === Wave 4 ===
        # ==============

        # === Non-Admin ===
        oadp-non-admin-oadp-dev) echo "migtools_oadp_non_admin_oadp-dev" ;;
        oadp-non-admin-oadp-1.5) echo "migtools_oadp_non_admin_oadp-1.5" ;;

        # === OpenShift Velero Plugin ===
        openshift-velero-plugin-oadp-dev) echo "openshift_openshift_velero_plugin_oadp-dev" ;;
        openshift-velero-plugin-oadp-1.5) echo "openshift_openshift_velero_plugin_oadp-1.5" ;;

        # === Wave 5 ===
        # ==============

        # === Must Gather ===
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

    if [ "$branch" = "oadp-dev" ]; then
        case "$wave" in
            1) echo "udistribution-main kopia-oadp-dev" ;;
            2) echo "velero-oadp-dev" ;;
            3) echo "velero-plugin-for-csi-oadp-dev oadp-operator-oadp-dev velero-plugin-for-aws-oadp-dev velero-plugin-for-legacy-aws-oadp-dev velero-plugin-for-microsoft-azure-oadp-dev" ;;
            4) echo "oadp-non-admin-oadp-dev openshift-velero-plugin-oadp-dev" ;;
            5) echo "oadp-must-gather-oadp-dev oadp-cli-oadp-dev" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.5" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.5" ;;
            2) echo "velero-1.5" ;;
            3) echo "velero-plugin-for-csi-1.5 oadp-operator-1.5 velero-plugin-for-aws-1.5 velero-plugin-for-legacy-aws-oadp-1.5 velero-plugin-for-microsoft-azure-1.5" ;;
            4) echo "oadp-non-admin-1.5 openshift-velero-plugin-1.5" ;;
            5) echo "oadp-must-gather-1.5" ;;
            *) return 1 ;;
        esac
    else
        return 1
    fi
}

# === Utility Functions ===

error_exit() {
    printf "❌  %s\n" "$*" >&2
    exit 1
}

log_section() {
    printf "\n==========================================\n%s\n==========================================\n" "$*"
}

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

Examples:
  $0 kopia-oadp-dev           # Run single repository oadp-dev receipt
  $0 kopia-oadp-1.5           # Run single repository oadp-1.5 receipt
  $0 kopia -b oadp-1.5        # Run single repository, same as above  
  $0 -w 1                     # Run wave 1
  $0 -d -w 2                  # Dry-run wave 2
  $0 -t kopia                 # Test configuration
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

    log_info "Loading ${source_type} config..."

    if [ "$source_type" = "local" ]; then
        [ -f "rebase-configs/$config_file" ] || error_exit "Config file not found: rebase-configs/${config_file}"
        . "rebase-configs/$config_file"
    else
        config_url="https://raw.githubusercontent.com/oadp-rebasebot/oadp-rebase/refs/heads/oadp-dev/rebase-configs/$config_file"
        temp_config="$(mktemp)"
        trap 'rm -f "$temp_config"' EXIT
        if ! curl --fail --silent --show-error "$config_url" > "$temp_config"; then
            error_exit "Failed to load remote config: ${config_url}"
        fi
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
    log_info "SOURCE_UPSTREAM_REPO: ${SOURCE_UPSTREAM_REPO:-<not set>}"
    log_info "DESTINATION_DOWNSTREAM_REPO: ${DESTINATION_DOWNSTREAM_REPO:-<not set>}"
    log_info "REBASE_REPO: ${REBASE_REPO:-<not set>}"
    # Check if HOOK_SCRIPTS is set and non-empty
    if [[ -n "${HOOK_SCRIPTS:-}" ]]; then
        log_info "HOOK_SCRIPTS: $HOOK_SCRIPTS"
    fi

    # Check if EXTRA_REBASEBOT_ARGS is set and non-empty
    if [[ -n "${EXTRA_REBASEBOT_ARGS:-}" ]]; then
        log_info "EXTRA_REBASEBOT_ARGS: $EXTRA_REBASEBOT_ARGS"
    fi    
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

    log_info "SOURCE_UPSTREAM_REPO: $SOURCE_UPSTREAM_REPO"
    log_info "DESTINATION_DOWNSTREAM_REPO: $DESTINATION_DOWNSTREAM_REPO"
    log_info "REBASE_REPO: $REBASE_REPO"

    CMD="$CONTAINER_ENGINE run --rm --pull=always \
  -v \"$SECRETS_DIR:/secrets:Z,ro\" \
  -e GIT_USERNAME=\"$GIT_USERNAME\" \
  -e GIT_EMAIL=\"$GIT_EMAIL\" \
  \"$REBASEBOT_IMAGE\" \
  --source \"$SOURCE_UPSTREAM_REPO\" \
  --dest \"$DESTINATION_DOWNSTREAM_REPO\" \
  --rebase \"$REBASE_REPO\" \
  --github-app-id \"$GITHUB_APP_ID\" \
  --github-app-key /secrets/oadp-rebasebot-app-key \
  --github-cloner-id \"$GITHUB_CLONER_ID\" \
  --github-cloner-key /secrets/oadp-rebasebot-cloner-key"

    # Check if HOOK_SCRIPTS is set and non-empty
    if [[ -n "${HOOK_SCRIPTS:-}" ]]; then
        log_info "HOOK_SCRIPTS: $HOOK_SCRIPTS"
        CMD="$CMD $HOOK_SCRIPTS"
    fi

    # Check if EXTRA_REBASEBOT_ARGS is set and non-empty
    if [[ -n "${EXTRA_REBASEBOT_ARGS:-}" ]]; then
        log_info "EXTRA_REBASEBOT_ARGS: $EXTRA_REBASEBOT_ARGS"
        CMD="$CMD $EXTRA_REBASEBOT_ARGS"
    fi

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

    failed=""
    skipped=""
    success_count=0

    for config in $repos; do
        log_section "Processing repo: $config"
        if ! get_config_name "$config" >/dev/null 2>&1; then
            log_warn "Skipping $config (no config)"
            skipped="$skipped $config"
            continue
        fi

        if run_container_rebase "$config" "$dry_run" "$source_type"; then
            log_success "Processed $config"
            success_count=$((success_count + 1))
        else
            log_fail "Failed $config"
            failed="$failed $config"
        fi
    done

    total_count=0
    failed_count=0
    skipped_count=0

    for repo in $repos; do total_count=$((total_count + 1)); done
    for repo in $failed; do [ -n "$repo" ] && failed_count=$((failed_count + 1)); done
    for repo in $skipped; do [ -n "$repo" ] && skipped_count=$((skipped_count + 1)); done

    log_section "Wave $wave_num summary"
    log_info "Total: ${total_count}, Success: ${success_count}, Failed: ${failed_count}, Skipped: ${skipped_count}"

    if [ -n "$failed" ]; then
        log_fail "Failed repositories:"
        for repo in $failed; do [ -n "$repo" ] && printf "  %s\n" "$repo"; done
        return 1
    fi

    log_success "All repositories processed successfully!"
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
        -b|--branch) OADP_BRANCH="$2"; shift 2 ;;
        -s|--secrets-dir) SECRETS_DIR="$2"; shift 2 ;;
        -*) error_exit "Unknown option: $1" ;;
        *) [ -z "$TARGET" ] || error_exit "Multiple targets specified"; TARGET="$1"; shift ;;
    esac
done

[ -n "$TARGET" ] || { usage; error_exit "Target is required"; }

SOURCE_TYPE="local"
[ "$REMOTE_MODE" = "true" ] && SOURCE_TYPE="remote"

if [ "$WAVE_MODE" = "true" ]; then
    [ "$TEST_MODE" = "true" ] && error_exit "Test mode not supported for wave"
    run_wave "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
else
    if ! get_config_name "$TARGET" >/dev/null 2>&1; then
        TARGET="${TARGET}-${OADP_BRANCH}"
    fi

    get_config_name "$TARGET" >/dev/null || error_exit "Unknown config '$TARGET'"

    if [ "$TEST_MODE" = "true" ]; then
        test_config "$TARGET"
    else
        run_container_rebase "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
    fi
fi

