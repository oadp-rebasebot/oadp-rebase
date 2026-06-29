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
WORKING_DIR=""

# === Repository Configuration Mapping ===

get_config_name() {
    case "$1" in
        # === Udistribution ===
        udistribution-main) echo "migtools_udistribution_main" ;;
        kubevirt-velero-plugin-main) echo "migtools_kubevirt_velero_plugin_main" ;;
        kubevirt-velero-plugin-oadp-1.3) echo "migtools_kubevirt_velero_plugin_oadp-1.3" ;;
        kubevirt-velero-plugin-oadp-1.4) echo "migtools_kubevirt_velero_plugin_oadp-1.4" ;;
        kubevirt-velero-plugin-oadp-1.5) echo "migtools_kubevirt_velero_plugin_oadp-1.5" ;;
        kubevirt-velero-plugin-oadp-1.6) echo "migtools_kubevirt_velero_plugin_oadp-1.6" ;;

        # === Wave 1 ===
        kopia-oadp-dev) echo "migtools_kopia_oadp-dev" ;;
        kopia-oadp-1.3) echo "migtools_kopia_oadp-1.3" ;;
        kopia-oadp-1.4) echo "migtools_kopia_oadp-1.4" ;;
        kopia-oadp-1.5) echo "migtools_kopia_oadp-1.5" ;;
        kopia-oadp-1.6) echo "migtools_kopia_oadp-1.6" ;;
        restic-oadp-dev) echo "openshift_restic_oadp-dev" ;;
        restic-oadp-1.3) echo "openshift_restic_oadp-1.3" ;;
        restic-oadp-1.4) echo "openshift_restic_oadp-1.4" ;;
        restic-oadp-1.5) echo "openshift_restic_oadp-1.5" ;;
        restic-oadp-1.6) echo "openshift_restic_oadp-1.6" ;;
        filebrowser-oadp-dev) echo "migtools_filebrowser_oadp-dev" ;;
        filebrowser-oadp-1.6) echo "migtools_filebrowser_oadp-1.6" ;;

        # === Wave 2 ===
        velero-oadp-dev) echo "openshift_velero_oadp-dev" ;;
        velero-oadp-1.3) echo "openshift_velero_oadp-1.3" ;;
        velero-oadp-1.4) echo "openshift_velero_oadp-1.4" ;;
        velero-oadp-1.5) echo "openshift_velero_oadp-1.5" ;;
        velero-oadp-1.6) echo "openshift_velero_oadp-1.6" ;;

        # === Wave 3 ===
        velero-plugin-for-csi-oadp-dev) echo "openshift_velero_plugin_for_csi_oadp-dev" ;;
        velero-plugin-for-csi-oadp-1.3) echo "openshift_velero_plugin_for_csi_oadp-1.3" ;;
        oadp-operator-oadp-dev) echo "openshift_oadp-operator_oadp-dev" ;;
        oadp-operator-oadp-1.3) echo "openshift_oadp-operator_oadp-1.3" ;;
        oadp-operator-oadp-1.4) echo "openshift_oadp-operator_oadp-1.4" ;;
        oadp-operator-oadp-1.5) echo "openshift_oadp-operator_oadp-1.5" ;;
        oadp-operator-oadp-1.6) echo "openshift_oadp-operator_oadp-1.6" ;;
        velero-plugin-for-aws-oadp-dev) echo "openshift_velero_plugin_for_aws_oadp-dev" ;;
        velero-plugin-for-aws-oadp-1.3) echo "openshift_velero_plugin_for_aws_oadp-1.3" ;;
        velero-plugin-for-aws-oadp-1.4) echo "openshift_velero_plugin_for_aws_oadp-1.4" ;;
        velero-plugin-for-aws-oadp-1.5) echo "openshift_velero_plugin_for_aws_oadp-1.5" ;;
        velero-plugin-for-aws-oadp-1.6) echo "openshift_velero_plugin_for_aws_oadp-1.6" ;;
        velero-plugin-for-legacy-aws-oadp-dev) echo "openshift_velero_plugin_for_legacy_aws_oadp-dev" ;;
        velero-plugin-for-legacy-aws-oadp-1.4) echo "openshift_velero_plugin_for_legacy_aws_oadp-1.4" ;;
        velero-plugin-for-legacy-aws-oadp-1.5) echo "openshift_velero_plugin_for_legacy_aws_oadp-1.5" ;;
        velero-plugin-for-legacy-aws-oadp-1.6) echo "openshift_velero_plugin_for_legacy_aws_oadp-1.6" ;;
        velero-plugin-for-microsoft-azure-oadp-dev) echo "openshift_velero_plugin_for_microsoft_azure_oadp-dev" ;;
        velero-plugin-for-microsoft-azure-oadp-1.3) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.3" ;;
        velero-plugin-for-microsoft-azure-oadp-1.4) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.4" ;;
        velero-plugin-for-microsoft-azure-oadp-1.5) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.5" ;;
        velero-plugin-for-microsoft-azure-oadp-1.6) echo "openshift_velero_plugin_for_microsoft_azure_oadp-1.6" ;;
        velero-plugin-for-gcp-oadp-dev) echo "openshift_velero_plugin_for_gcp_oadp-dev" ;;
        velero-plugin-for-gcp-oadp-1.3) echo "openshift_velero_plugin_for_gcp_oadp-1.3" ;;
        velero-plugin-for-gcp-oadp-1.4) echo "openshift_velero_plugin_for_gcp_oadp-1.4" ;;
        velero-plugin-for-gcp-oadp-1.5) echo "openshift_velero_plugin_for_gcp_oadp-1.5" ;;
        velero-plugin-for-gcp-oadp-1.6) echo "openshift_velero_plugin_for_gcp_oadp-1.6" ;;

        # === Wave 4 ===
        oadp-non-admin-oadp-dev) echo "migtools_oadp_non_admin_oadp-dev" ;;
        oadp-non-admin-oadp-1.4) echo "migtools_oadp_non_admin_oadp-1.4" ;;
        oadp-non-admin-oadp-1.5) echo "migtools_oadp_non_admin_oadp-1.5" ;;
        oadp-non-admin-oadp-1.6) echo "migtools_oadp_non_admin_oadp-1.6" ;;
        oadp-vm-file-restore-oadp-dev) echo "migtools_oadp_vm_file_restore_oadp-dev" ;;
        oadp-vm-file-restore-oadp-1.6) echo "migtools_oadp_vm_file_restore_oadp-1.6" ;;
        openshift-velero-plugin-oadp-dev) echo "openshift_openshift_velero_plugin_oadp-dev" ;;
        openshift-velero-plugin-oadp-1.3) echo "openshift_openshift_velero_plugin_oadp-1.3" ;;
        openshift-velero-plugin-oadp-1.4) echo "openshift_openshift_velero_plugin_oadp-1.4" ;;
        openshift-velero-plugin-oadp-1.5) echo "openshift_openshift_velero_plugin_oadp-1.5" ;;
        openshift-velero-plugin-oadp-1.6) echo "openshift_openshift_velero_plugin_oadp-1.6" ;;
        kubevirt-datamover-controller-oadp-dev) echo "migtools_kubevirt_datamover_controller_oadp-dev" ;;
        kubevirt-datamover-controller-oadp-1.6) echo "migtools_kubevirt_datamover_controller_oadp-1.6" ;;

        # === Wave 5 ===
        oadp-must-gather-oadp-dev) echo "openshift_oadp_must_gather_oadp-dev" ;;
        oadp-must-gather-oadp-1.3) echo "openshift_oadp_must_gather_oadp-1.3" ;;
        oadp-must-gather-oadp-1.4) echo "openshift_oadp_must_gather_oadp-1.4" ;;
        oadp-must-gather-oadp-1.5) echo "openshift_oadp_must_gather_oadp-1.5" ;;
        oadp-must-gather-oadp-1.6) echo "openshift_oadp_must_gather_oadp-1.6" ;;
        kubevirt-datamover-plugin-oadp-dev) echo "migtools_kubevirt_datamover_plugin_oadp-dev" ;;
        kubevirt-datamover-plugin-oadp-1.6) echo "migtools_kubevirt_datamover_plugin_oadp-1.6" ;;

        # === OADP CLI ===
        oadp-cli-oadp-dev) echo "migtools_oadp_cli_oadp-dev" ;;
        oadp-cli-oadp-1.4) echo "migtools_oadp_cli_oadp-1.4" ;;
        oadp-cli-oadp-1.5) echo "migtools_oadp_cli_oadp-1.5" ;;
        oadp-cli-oadp-1.6) echo "migtools_oadp_cli_oadp-1.6" ;;

        # === HyperShift OADP Plugin ===
        hypershift-oadp-plugin-main) echo "openshift_hypershift_oadp_plugin_main" ;;
        hypershift-oadp-plugin-oadp-1.5) echo "openshift_hypershift_oadp_plugin_oadp-1.5" ;;
        hypershift-oadp-plugin-oadp-1.6) echo "openshift_hypershift_oadp_plugin_oadp-1.6" ;;

        # === OADP VMDP ===
        oadp-vmdp-oadp-dev) echo "migtools_oadp_vmdp_oadp-dev" ;;
        oadp-vmdp-oadp-1.6) echo "migtools_oadp_vmdp_oadp-1.6" ;;

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
            echo "udistribution-main kopia-oadp-dev restic-oadp-dev filebrowser-oadp-dev oadp-vmdp-oadp-dev"
            return 0
        fi
    fi

    if [ "$branch" = "oadp-dev" ]; then
        case "$wave" in
            2) echo "velero-oadp-dev" ;;
            3) echo "kubevirt-velero-plugin-main velero-plugin-for-csi-oadp-dev oadp-operator-oadp-dev velero-plugin-for-aws-oadp-dev velero-plugin-for-legacy-aws-oadp-dev velero-plugin-for-microsoft-azure-oadp-dev velero-plugin-for-gcp-oadp-dev hypershift-oadp-plugin-main" ;;
            4) echo "oadp-non-admin-oadp-dev openshift-velero-plugin-oadp-dev kubevirt-datamover-controller-oadp-dev oadp-vm-file-restore-oadp-dev" ;;
            5) echo "oadp-must-gather-oadp-dev oadp-cli-oadp-dev kubevirt-datamover-plugin-oadp-dev" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.3" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.3 restic-oadp-1.3" ;;
            2) echo "velero-oadp-1.3" ;;
            3) echo "kubevirt-velero-plugin-oadp-1.3 velero-plugin-for-csi-oadp-1.3 oadp-operator-oadp-1.3 velero-plugin-for-aws-oadp-1.3 velero-plugin-for-gcp-oadp-1.3 velero-plugin-for-microsoft-azure-oadp-1.3" ;;
            4) echo "openshift-velero-plugin-oadp-1.3" ;;
            5) echo "oadp-must-gather-oadp-1.3" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.4" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.4 restic-oadp-1.4" ;;
            2) echo "velero-oadp-1.4" ;;
            3) echo "kubevirt-velero-plugin-oadp-1.4 oadp-operator-oadp-1.4 velero-plugin-for-aws-oadp-1.4 velero-plugin-for-legacy-aws-oadp-1.4 velero-plugin-for-gcp-oadp-1.4 velero-plugin-for-microsoft-azure-oadp-1.4" ;;
            4) echo "oadp-non-admin-oadp-1.4 openshift-velero-plugin-oadp-1.4" ;;
            5) echo "oadp-must-gather-oadp-1.4 oadp-cli-oadp-1.4" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.5" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.5 restic-oadp-1.5" ;;
            2) echo "velero-oadp-1.5" ;;
            3) echo "kubevirt-velero-plugin-oadp-1.5 oadp-operator-oadp-1.5 velero-plugin-for-aws-oadp-1.5 velero-plugin-for-legacy-aws-oadp-1.5 velero-plugin-for-microsoft-azure-oadp-1.5 velero-plugin-for-gcp-oadp-1.5 hypershift-oadp-plugin-oadp-1.5" ;;
            4) echo "oadp-non-admin-oadp-1.5 openshift-velero-plugin-oadp-1.5" ;;
            5) echo "oadp-must-gather-oadp-1.5 oadp-cli-oadp-1.5" ;;
            *) return 1 ;;
        esac
    elif [ "$branch" = "oadp-1.6" ]; then
        case "$wave" in
            1) echo "kopia-oadp-1.6 restic-oadp-1.6 filebrowser-oadp-1.6 oadp-vmdp-oadp-1.6" ;;
            2) echo "velero-oadp-1.6" ;;
            3) echo "kubevirt-velero-plugin-oadp-1.6 oadp-operator-oadp-1.6 velero-plugin-for-aws-oadp-1.6 velero-plugin-for-legacy-aws-oadp-1.6 velero-plugin-for-microsoft-azure-oadp-1.6 velero-plugin-for-gcp-oadp-1.6 hypershift-oadp-plugin-oadp-1.6" ;;
            4) echo "oadp-non-admin-oadp-1.6 openshift-velero-plugin-oadp-1.6 kubevirt-datamover-controller-oadp-1.6 oadp-vm-file-restore-oadp-1.6" ;;
            5) echo "oadp-must-gather-oadp-1.6 oadp-cli-oadp-1.6 kubevirt-datamover-plugin-oadp-1.6" ;;
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

get_repo_name() {
    config="$1"
    # Extract the repository name (first part before the branch suffix)
    # e.g., "kopia-oadp-dev" -> "kopia"
    # e.g., "velero-plugin-for-aws-oadp-1.5" -> "velero-plugin-for-aws"
    # e.g., "udistribution-main" -> "udistribution"
    echo "$config" | sed -E 's/-(oadp-dev|oadp-1\.[0-9]+|main)$//'
}

usage() {
    cat <<EOF
OADP Rebase Runner

Usage: $0 [OPTIONS] <target>

Arguments:
  target    Repository (exact repo-branch) or wave number

Options:
  -d, --dry-run              Dry-run mode
  -t, --test                 Test configuration only (local only)
  -b, --branch BRANCH        Specify branch (default: $OADP_BRANCH)
  -w, --wave                 Execute entire wave
  -s, --secrets-dir DIR      Secrets directory
  -r, --remote               Use remote configuration
  -l, --local                Use local rebasebot CLI instead of container
      --working-dir DIR      Working directory for rebase operations
      --local-hooks          Use local hook scripts from ./rebasebot-hook-scripts
      --conflict-policy POL  Rebasebot conflict policy (default: strict)
  -h, --help                 Show this help
EOF
}

ensure_working_dir() {
    if [ -n "$WORKING_DIR" ]; then
        mkdir -p "$WORKING_DIR" || error_exit "Failed to create working dir: $WORKING_DIR"
        chmod 777 "$WORKING_DIR"
        log_info "Using working directory: $WORKING_DIR"
    fi
}

transform_hook_scripts_to_local() {
    mode="$1"  # "container" or "cli"

    if [ -z "${HOOK_SCRIPTS:-}" ]; then
        return 0
    fi

    if [ "$mode" = "container" ]; then
        # For container: replace git: URLs with /hooks/ paths
        HOOK_SCRIPTS=$(echo "$HOOK_SCRIPTS" | sed -E 's|git:https://[^:]+:rebasebot-hook-scripts/|/hooks/|g')
    else
        # For CLI: replace git: URLs with absolute paths to local directory
        local_hooks_dir="$(pwd)/rebasebot-hook-scripts"
        HOOK_SCRIPTS=$(echo "$HOOK_SCRIPTS" | sed -E "s|git:https://[^:]+:rebasebot-hook-scripts/|${local_hooks_dir}/|g")
    fi
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
    
    # Load version-specific variables from the SSOT before sourcing the config
    oadp_version=$(echo "$config" | grep -oE 'oadp-1\.[0-9]+' | head -1)
    if [ -n "$oadp_version" ]; then
        versions_file="versions/${oadp_version}.env"
        if [ "$source_type" = "local" ]; then
            if [ -f "$versions_file" ]; then
                . "$versions_file"
                log_info "Loaded versions: $versions_file"
            fi
        else
            versions_url="https://raw.githubusercontent.com/oadp-rebasebot/oadp-rebase/refs/heads/oadp-dev/$versions_file"
            temp_versions="$(mktemp)"
            if curl --fail --silent --show-error "$versions_url" > "$temp_versions" 2>/dev/null; then
                . "$temp_versions"
                log_info "Loaded remote versions: $versions_file"
            fi
            rm -f "$temp_versions"
        fi
    fi

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

print_config() {
    log_info "Rebase details:"
    log_info "  🌊  Upstream:           ${SOURCE_UPSTREAM_REPO:-<not set>}"
    log_info "    🔀 Rebase (for PR):   ${REBASE_REPO:-<not set>}"
    log_info "      🎯 Downstream:      ${DESTINATION_DOWNSTREAM_REPO:-<not set>}"
    [ -n "${HOOK_SCRIPTS:-}" ] && log_info "  🪝 HOOK_SCRIPTS: $HOOK_SCRIPTS"
    [ -n "${EXTRA_REBASEBOT_ARGS:-}" ] && log_info "  🔧 EXTRA_REBASEBOT_ARGS: $EXTRA_REBASEBOT_ARGS"
    return 0
}

test_config() {
    config="$1"
    log_info "Testing local config: $config"
    load_config "$config" "local"

    # Transform hook scripts to local paths if --local-hooks is set
    if [ "$USE_LOCAL_HOOKS" = "true" ]; then
        [ -d "./rebasebot-hook-scripts" ] || error_exit "Local hooks directory ./rebasebot-hook-scripts not found"
        log_info "Using local hook scripts from ./rebasebot-hook-scripts"
        transform_hook_scripts_to_local "cli"
    fi

    # Check if this repo should be skipped
    if [ "${SKIP_REPO:-false}" = "true" ]; then
        log_warn "Skipping $config (SKIP_REPO=true in config)"
        return 0
    fi
    print_config
}

run_local_rebase() {
    config="$1"
    dry_run="$2"
    source_type="$3"

    # Check if rebasebot CLI is available
    if ! command -v rebasebot >/dev/null 2>&1; then
        error_exit "rebasebot CLI not found in PATH. Please install it or use container mode (remove --local flag)."
    fi

    log_section "Rebasing using receipt: $config (local CLI)"
    log_info "Dry run: $dry_run, Config source: $source_type"

    check_secrets
    ensure_working_dir
    load_config "$config" "$source_type"

    # Transform hook scripts to local paths if --local-hooks is set
    if [ "$USE_LOCAL_HOOKS" = "true" ]; then
        [ -d "./rebasebot-hook-scripts" ] || error_exit "Local hooks directory ./rebasebot-hook-scripts not found"
        log_info "Using local hook scripts from ./rebasebot-hook-scripts"
        transform_hook_scripts_to_local "cli"
    fi

    print_config

    # Check if this repo should be skipped
    if [ "${SKIP_REPO:-false}" = "true" ]; then
        log_warn "Skipping $config (SKIP_REPO=true in config)"
        return 0
    fi

    # Determine repository-specific working directory
    REPO_WORKING_DIR=""
    if [ -n "$WORKING_DIR" ]; then
        repo_name="$(get_repo_name "$config")"
        REPO_WORKING_DIR="${WORKING_DIR}/${repo_name}"
        mkdir -p "$REPO_WORKING_DIR" || error_exit "Failed to create repo working dir: $REPO_WORKING_DIR"
        chmod 777 "$REPO_WORKING_DIR"
        log_info "Using repository working directory: $REPO_WORKING_DIR"
    fi

    CMD="rebasebot \
  --conflict-policy \"$CONFLICT_POLICY\" \
  --source \"$SOURCE_UPSTREAM_REPO\" \
  --dest \"$DESTINATION_DOWNSTREAM_REPO\" \
  --rebase \"$REBASE_REPO\" \
  --git-username \"$GIT_USERNAME\" \
  --git-email \"$GIT_EMAIL\" \
  --github-app-id \"$GITHUB_APP_ID\" \
  --github-app-key \"$SECRETS_DIR/oadp-rebasebot-app-key\" \
  --github-cloner-id \"$GITHUB_CLONER_ID\" \
  --github-cloner-key \"$SECRETS_DIR/oadp-rebasebot-cloner-key\""

    [ -n "$REPO_WORKING_DIR" ] && CMD="$CMD --working-dir \"$REPO_WORKING_DIR\""
    [ -n "${HOOK_SCRIPTS:-}" ] && CMD="$CMD $HOOK_SCRIPTS"
    [ -n "${EXTRA_REBASEBOT_ARGS:-}" ] && CMD="$CMD $EXTRA_REBASEBOT_ARGS"
    [ "$dry_run" = "true" ] && CMD="$CMD --dry-run"

    log_info "Command:"
    log_info "\$ ${CMD}"
    sh -c "$CMD"
}

run_container_rebase() {
    config="$1"
    dry_run="$2"
    source_type="$3"

    CONTAINER_ENGINE="$(command -v podman || true)"
    [ -z "$CONTAINER_ENGINE" ] && CONTAINER_ENGINE="$(command -v docker || true)"
    [ -z "$CONTAINER_ENGINE" ] && error_exit "No podman or docker found"

    log_section "Rebasing using receipt: $config"
    log_info "Dry run: $dry_run, Config source: $source_type"

    check_secrets
    ensure_working_dir
    load_config "$config" "$source_type"

    # Transform hook scripts to local paths and setup mount if --local-hooks is set
    HOOKS_MOUNT=""
    if [ "$USE_LOCAL_HOOKS" = "true" ]; then
        [ -d "./rebasebot-hook-scripts" ] || error_exit "Local hooks directory ./rebasebot-hook-scripts not found"
        log_info "Using local hook scripts from ./rebasebot-hook-scripts"
        transform_hook_scripts_to_local "container"
        # Mount the local hooks directory into the container
        HOOKS_MOUNT="-v \"$(pwd)/rebasebot-hook-scripts:/hooks:Z,ro\""
    fi

    print_config

    # Check if this repo should be skipped
    if [ "${SKIP_REPO:-false}" = "true" ]; then
        log_warn "Skipping $config (SKIP_REPO=true in config)"
        return 0
    fi

    # Determine repository-specific working directory
    WORKING_MOUNT=""
    REBASEBOT_WORKING_DIR=""
    if [ -n "$WORKING_DIR" ]; then
        repo_name="$(get_repo_name "$config")"

        # Create the parent working directory and repo-specific subdirectory
        mkdir -p "${WORKING_DIR}/${repo_name}" || error_exit "Failed to create repo working dir: ${WORKING_DIR}/${repo_name}"
        chmod 777 "${WORKING_DIR}/${repo_name}"

        # Mount the parent working directory
        WORKING_MOUNT="-v \"$WORKING_DIR:/working:Z,rw\""

        # Pass the repo-specific subdirectory to rebasebot (as seen inside container)
        REBASEBOT_WORKING_DIR="/working/${repo_name}"

        log_info "Using repository working directory: ${WORKING_DIR}/${repo_name} (mounted as ${REBASEBOT_WORKING_DIR})"
    fi

    # Add --userns=keep-id and --user for podman to preserve host UID/GID and prevent ownership issues
    USERNS_FLAG=""
    USER_FLAG=""
    EXTRA_ENV_FLAGS=""
    if echo "$CONTAINER_ENGINE" | grep -q "podman"; then
        USERNS_FLAG="--userns=keep-id"
        USER_FLAG="--user $(id -u):$(id -g)"
        # Configure git to trust all directories to avoid "dubious ownership" errors
        # Set Go cache/module directories to /tmp to avoid permission issues
        # Set HOME to /tmp so various tools can write config files
        EXTRA_ENV_FLAGS="-e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0='*' -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -e HOME=/tmp"
    fi

    CMD="$CONTAINER_ENGINE run --rm --pull=always $USERNS_FLAG $USER_FLAG \
  -v \"$SECRETS_DIR:/secrets:Z,ro\" $WORKING_MOUNT $HOOKS_MOUNT \
  -e GIT_USERNAME=\"$GIT_USERNAME\" \
  -e GIT_EMAIL=\"$GIT_EMAIL\" \
  ${GO_VET_TAGS:+-e GO_VET_TAGS=\"$GO_VET_TAGS\"} \
  $EXTRA_ENV_FLAGS \
  \"$REBASEBOT_IMAGE\" \
  --conflict-policy \"$CONFLICT_POLICY\" \
  --source \"$SOURCE_UPSTREAM_REPO\" \
  --dest \"$DESTINATION_DOWNSTREAM_REPO\" \
  --rebase \"$REBASE_REPO\" \
  --git-username \"$GIT_USERNAME\" \
  --git-email \"$GIT_EMAIL\" \
  --github-app-id \"$GITHUB_APP_ID\" \
  --github-app-key /secrets/oadp-rebasebot-app-key \
  --github-cloner-id \"$GITHUB_CLONER_ID\" \
  --github-cloner-key /secrets/oadp-rebasebot-cloner-key"

    [ -n "$REBASEBOT_WORKING_DIR" ] && CMD="$CMD --working-dir \"$REBASEBOT_WORKING_DIR\""
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
    pr_summary_file="$(mktemp)"
    rebase_output_file="$(mktemp)"

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
        if [ "$USE_LOCAL_CLI" = "true" ]; then
            rebase_func="run_local_rebase"
        else
            rebase_func="run_container_rebase"
        fi

        rebase_rc_file="$(mktemp)"
        ( set +e; $rebase_func "$config" "$dry_run" "$source_type" 2>&1; echo $? > "$rebase_rc_file" ) | tee "$rebase_output_file"
        rebase_rc="$(cat "$rebase_rc_file" 2>/dev/null || echo 1)"
        rm -f "$rebase_rc_file"

        # Extract PR status messages from output (regardless of success/failure)
        grep -E '(I created a new rebase PR|I updated existing rebase PR|PR .+/pull/[0-9]+ already contains|rebase/manual)' \
            "$rebase_output_file" | sed 's/.*INFO - //' >> "$pr_summary_file" || true

        if [ "$rebase_rc" = "0" ]; then
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
    else
        log_success "All repositories processed (or skipped) successfully!"
    fi

    if [ -s "$pr_summary_file" ]; then
        printf "\n"
        log_info "Pull request results:"
        while IFS= read -r line; do
            printf "  🔗 %s\n" "$line"
        done < "$pr_summary_file"
    fi

    rm -f "$rebase_output_file" "$pr_summary_file"

    [ -n "$failed" ] && return 1
}

# === Argument Parsing ===

DRY_RUN="false"
TEST_MODE="false"
WAVE_MODE="false"
REMOTE_MODE="false"
USE_LOCAL_CLI="false"
USE_LOCAL_HOOKS="false"
CONFLICT_POLICY="strict"
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        -d|--dry-run) DRY_RUN="true"; shift ;;
        -t|--test) TEST_MODE="true"; shift ;;
        -w|--wave) WAVE_MODE="true"; shift ;;
        -r|--remote) REMOTE_MODE="true"; shift ;;
        -l|--local) USE_LOCAL_CLI="true"; shift ;;
        -b|--branch) OADP_BRANCH="$2"; OADP_BRANCH_SET=1; shift 2 ;;
        -s|--secrets-dir) SECRETS_DIR="$2"; shift 2 ;;
        --working-dir) WORKING_DIR="$2"; shift 2 ;;
        --local-hooks) USE_LOCAL_HOOKS="true"; shift ;;
        --conflict-policy) CONFLICT_POLICY="$2"; shift 2 ;;
        -*) error_exit "Unknown option: $1" ;;
        *) [ -z "$TARGET" ] || error_exit "Multiple targets specified"; TARGET="$1"; shift ;;
    esac
done

[ -n "$TARGET" ] || { usage; error_exit "Target is required"; }

SOURCE_TYPE="local"
[ "$REMOTE_MODE" = "true" ] && SOURCE_TYPE="remote"

# Special handling for udistribution and kubevirt-velero-plugin default branch
if [ "$WAVE_MODE" != "true" ]; then
    if [ "$TARGET" = "udistribution" ] && [ -z "${OADP_BRANCH_SET:-}" ]; then
        TARGET="udistribution-main"
    elif [ "$TARGET" = "kubevirt-velero-plugin" ] && [ -z "${OADP_BRANCH_SET:-}" ]; then
        TARGET="kubevirt-velero-plugin-main"
    elif [ "$TARGET" = "hypershift-oadp-plugin" ] && [ -z "${OADP_BRANCH_SET:-}" ]; then
        TARGET="hypershift-oadp-plugin-main"
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
        if [ "$USE_LOCAL_CLI" = "true" ]; then
            run_local_rebase "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
        else
            run_container_rebase "$TARGET" "$DRY_RUN" "$SOURCE_TYPE"
        fi
    fi
fi

