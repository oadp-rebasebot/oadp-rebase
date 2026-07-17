#!/bin/sh
#
# Deterministic conflict triage for rebasebot failures.
#
# Reads rebasebot output from stdin, checks if all WARNING'd files are
# covered by hooks configured in the given config file or by the repo's
# expected_conflicts allowlist in repos.yaml. Outputs a JSON verdict to
# stdout. Exit 0 = safe to retry with --conflict-policy warn,
# exit 1 = needs human review.
#
# Usage: cat rebase-output.txt | conflict-triage.sh <config-file> [repo-name]
#
# Rule table — to add a new rule, add a case arm below and a test fixture.
#
#   File pattern                          Safe if
#   ------------------------------------  --------------------------------
#   go.mod, go.sum                        go-mod-tidy-and-commit.sh hook
#   Dockerfile, Dockerfile-Windows,       normalize-dockerfiles-and-commit.sh
#     hack/build-image/Dockerfile
#   (listed in expected_conflicts)        repo allowlist in repos.yaml
#   (everything else)                     ALWAYS UNSAFE — needs human review
#
# Prerequisite: rebasebot must have failed with "ERROR - Manual intervention
# is needed". If the error is infrastructure (image pull, auth, network),
# the script returns unsafe so no retry is attempted.

set -eu

CONFIG_FILE="${1:-}"
REPO_NAME="${2:-}"
[ -n "$CONFIG_FILE" ] || { echo "Usage: $0 <config-file> [repo-name]" >&2; exit 2; }
[ -f "$CONFIG_FILE" ] || { echo "Config file not found: $CONFIG_FILE" >&2; exit 2; }

# Source the versions SSOT so config variable guards (e.g. ${AWS_PLUGIN_TAG:?})
# resolve correctly when this script runs outside the pipeline environment.
SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
oadp_version=$(basename "$CONFIG_FILE" | grep -oE '(oadp-(1\.[0-9]+|dev))' | tail -1)
if [ -n "$oadp_version" ]; then
    versions_file="${SCRIPT_DIR}/versions/${oadp_version}.env"
    [ -f "$versions_file" ] && . "$versions_file"
fi

# Source the config to get HOOK_SCRIPTS
HOOK_SCRIPTS=""
. "$CONFIG_FILE"

# Load per-repo expected_conflicts allowlist from repos.yaml (if repo name given)
EXPECTED_CONFLICTS=""
if [ -n "$REPO_NAME" ] && command -v yq >/dev/null 2>&1; then
    REPOS_YAML="${SCRIPT_DIR}/repos.yaml"
    if [ -f "$REPOS_YAML" ]; then
        EXPECTED_CONFLICTS=$(yq -r \
            ".repos[] | select(.repo == \"${REPO_NAME}\") | .expected_conflicts // [] | .[]" \
            "$REPOS_YAML" 2>/dev/null || echo "")
    fi
fi

# Read rebasebot output from stdin
rebase_output=$(cat)

# Only triage if rebasebot actually failed due to conflict policy.
# The signature is "ERROR - Manual intervention is needed". If that line
# is missing, rebasebot failed for an infrastructure reason (image pull,
# auth, network) and retrying with warn won't help.
if ! echo "$rebase_output" | grep -q "^ERROR - Manual intervention is needed"; then
    printf '{"safe": false, "reason": "Rebasebot did not fail due to conflict policy", "affected_files": []}\n'
    exit 1
fi

# Extract unique filenames from WARNING lines
# Pattern: WARNING - Upstream content may have been dropped from 'FILENAME' by cherry-pick
warned_files=$(echo "$rebase_output" | grep "^WARNING - Upstream content may have been dropped from" | sed "s/.*from '\\([^']*\\)'.*/\\1/" | sort -u)

if [ -z "$warned_files" ]; then
    printf '{"safe": false, "reason": "Manual intervention required but no specific file warnings found", "affected_files": []}\n'
    exit 1
fi

has_go_mod_tidy=false
has_normalize_dockerfiles=false
echo "$HOOK_SCRIPTS" | grep -q "go-mod-tidy-and-commit.sh" && has_go_mod_tidy=true
echo "$HOOK_SCRIPTS" | grep -q "normalize-dockerfiles-and-commit.sh" && has_normalize_dockerfiles=true

# Helper: check if a file is in the expected_conflicts allowlist
is_expected_conflict() {
    _file="$1"
    if [ -n "$EXPECTED_CONFLICTS" ]; then
        echo "$EXPECTED_CONFLICTS" | grep -qxF "$_file" && return 0
    fi
    return 1
}

safe=true
reason=""
files_json="["
first=true

for file in $warned_files; do
    covered=false
    hook_name=""

    case "$file" in
        go.mod|go.sum)
            if [ "$has_go_mod_tidy" = "true" ]; then
                covered=true
                hook_name="go-mod-tidy-and-commit.sh"
            fi
            ;;
        Dockerfile|Dockerfile-Windows|hack/build-image/Dockerfile)
            if [ "$has_normalize_dockerfiles" = "true" ]; then
                covered=true
                hook_name="normalize-dockerfiles-and-commit.sh"
            fi
            ;;
    esac

    # If not covered by a hook, check the per-repo expected_conflicts allowlist
    if [ "$covered" = "false" ] && is_expected_conflict "$file"; then
        covered=true
        hook_name="expected_conflicts"
    fi

    if [ "$first" = "true" ]; then
        first=false
    else
        files_json="${files_json},"
    fi
    files_json="${files_json}{\"file\":\"${file}\",\"covered_by_hook\":${covered},\"hook_name\":\"${hook_name}\"}"

    if [ "$covered" = "false" ]; then
        safe=false
        if [ -z "$reason" ]; then
            reason="'${file}' is not covered by any configured hook"
        fi
    fi
done

files_json="${files_json}]"

if [ "$safe" = "true" ]; then
    reason="All conflicting files are covered by configured hooks"
fi

printf '{"safe": %s, "reason": "%s", "affected_files": %s}\n' "$safe" "$reason" "$files_json"

if [ "$safe" = "true" ]; then
    exit 0
else
    exit 1
fi
