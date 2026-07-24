#!/bin/bash
set -euo pipefail

# Generates go-replace_velero_oadp-*.sh hook scripts from versions/*.env.
# Each hook gets its downstream branch and upstream tag from the SSOT.
# Run: make generate

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_DIR="$REPO_ROOT/versions"
HOOKS_DIR="$REPO_ROOT/rebasebot-hook-scripts"

for versions_file in "$VERSIONS_DIR"/oadp-*.env; do
    [ -f "$versions_file" ] || continue

    unset OADP_BRANCH VELERO_UPSTREAM_TAG
    . "$versions_file"

    [ -z "${OADP_BRANCH:-}" ] && continue
    [ -z "${VELERO_UPSTREAM_TAG:-}" ] && continue

    hook_file="$HOOKS_DIR/go-replace_velero_${OADP_BRANCH}.sh"

    # Only update the require version when the upstream tag is semver (starts with v).
    # oadp-dev tracks "main", which isn't a valid Go module version.
    if [[ "$VELERO_UPSTREAM_TAG" == v* ]]; then
        REQUIRE_BLOCK="
# Update the require version to match the upstream tag from the SSOT.
go mod edit -require=\"\${UPSTREAM_MODULE}@${VELERO_UPSTREAM_TAG}\""
    else
        REQUIRE_BLOCK=""
    fi

    cat > "$hook_file" <<HOOK
#!/bin/bash
set -euo pipefail

# Generated from versions/${OADP_BRANCH}.env — do not edit manually.
# Run: make generate

DOWNSTREAM_BRANCH="${OADP_BRANCH}"
DOWNSTREAM_MODULE="github.com/openshift/velero"
GO_MOD_FILE="go.mod"

# Detect which velero module path the project uses (vmware-tanzu or velero-io)
if grep -qE "require.*github\\.com/velero-io/velero " "\$GO_MOD_FILE" || \\
   grep -qE "^\\s+github\\.com/velero-io/velero " "\$GO_MOD_FILE"; then
    UPSTREAM_MODULE="github.com/velero-io/velero"
else
    UPSTREAM_MODULE="github.com/vmware-tanzu/velero"
fi

# --- Step 1: go mod edit (must run BEFORE the sed replace) ---
# go mod edit validates the entire go.mod. The sed replace in step 2 writes a
# branch ref (e.g. "oadp-1.6") which is not valid semver. If go mod edit ran
# after sed, it would reject the file. So we run it first while the existing
# replace still has a valid pseudo-version.
${REQUIRE_BLOCK}

# Exclude kcp monorepo broken pseudo-version.
go mod edit -exclude=github.com/kcp-dev/kcp/sdk@v0.0.0-00010101000000-000000000000

# --- Step 2: sed replace (writes branch ref, resolved later by go mod tidy) ---
REPLACE_LINE="replace \$UPSTREAM_MODULE => \$DOWNSTREAM_MODULE \$DOWNSTREAM_BRANCH"

# Remove any stale replace for the other module path
if [ "\$UPSTREAM_MODULE" = "github.com/velero-io/velero" ]; then
    sed -i '/^replace github\\.com\\/vmware-tanzu\\/velero /d' "\$GO_MOD_FILE"
else
    sed -i '/^replace github\\.com\\/velero-io\\/velero /d' "\$GO_MOD_FILE"
fi

# Replace existing line or append if not present
if grep -q "^replace \$UPSTREAM_MODULE " "\$GO_MOD_FILE"; then
    sed -i "s|^replace \$UPSTREAM_MODULE .*|\$REPLACE_LINE|" "\$GO_MOD_FILE"
else
    echo "\$REPLACE_LINE" >> "\$GO_MOD_FILE"
fi

# --- Step 3: handle velero/pkg/apis sub-module ---
# Downstream velero uses pkg/apis as a separate Go sub-module with a local
# replace (=> ./pkg/apis). Consumers that replace the main velero module also
# need a replace for pkg/apis, otherwise go mod tidy fails trying to resolve
# the v0.0.0 pseudo-version at the upstream repo. Add it unconditionally —
# Go ignores replaces for modules not in the dependency graph.
APIS_UPSTREAM="\${UPSTREAM_MODULE}/pkg/apis"
APIS_DOWNSTREAM="\${DOWNSTREAM_MODULE}/pkg/apis"
APIS_REPLACE_LINE="replace \$APIS_UPSTREAM => \$APIS_DOWNSTREAM \$DOWNSTREAM_BRANCH"
if grep -q "^replace \$APIS_UPSTREAM " "\$GO_MOD_FILE"; then
    sed -i "s|^replace \$APIS_UPSTREAM .*|\$APIS_REPLACE_LINE|" "\$GO_MOD_FILE"
else
    echo "\$APIS_REPLACE_LINE" >> "\$GO_MOD_FILE"
fi
HOOK

    chmod +x "$hook_file"
    echo "Generated $hook_file"
done
