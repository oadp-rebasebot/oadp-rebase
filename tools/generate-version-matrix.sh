#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_DIR="$REPO_ROOT/versions"
OUTPUT_FILE="$REPO_ROOT/docs/version-matrix.md"

versions=()
for f in "$VERSIONS_DIR"/oadp-1.*.env; do
    [ -f "$f" ] || continue
    versions+=("$f")
done

if [ ${#versions[@]} -eq 0 ]; then
    echo "No versions files found in $VERSIONS_DIR" >&2
    exit 1
fi

tmp_output="$(mktemp)"
trap 'rm -f "$tmp_output"' EXIT

{
cat <<'HEADER'
<!-- Auto-generated from versions/*.env — do not edit manually. Run tools/generate-version-matrix.sh -->

# OADP Rebase Version Matrix

This document maps OADP versions to their upstream dependencies, tracks which repositories have downstream branches per version, and defines wave composition for the rebase process.

## Upstream Version Mapping

Each OADP release rebases against a specific set of upstream tags:

| OADP | Velero | Kopia | AWS Plugin | GCP Plugin | Azure Plugin | CSI Plugin | KubeVirt Plugin | Filebrowser |
|------|--------|-------|------------|------------|--------------|------------|-----------------|-------------|
HEADER

for f in "${versions[@]}"; do
    unset OADP_BRANCH VELERO_UPSTREAM_TAG KOPIA_UPSTREAM_TAG AWS_PLUGIN_TAG GCP_PLUGIN_TAG AZURE_PLUGIN_TAG CSI_PLUGIN_TAG KUBEVIRT_PLUGIN_TAG FILEBROWSER_TAG VELERO_TAG_SHA
    . "$f"
    ver="${OADP_BRANCH#oadp-}"
    echo "| $ver | $VELERO_UPSTREAM_TAG | $KOPIA_UPSTREAM_TAG | $AWS_PLUGIN_TAG | $GCP_PLUGIN_TAG | $AZURE_PLUGIN_TAG | ${CSI_PLUGIN_TAG:-n/a} | $KUBEVIRT_PLUGIN_TAG | ${FILEBROWSER_TAG:-n/a} |"
done

cat <<'MID1'

The CSI plugin was merged into Velero core after v1.12, so OADP 1.4+ no longer has a separate CSI plugin repository.

## Velero Tag SHAs

Used by `verify-tag-sha` pre-rebase hooks to detect silent tag recreation (supply chain risk):

| Velero Tag | Commit SHA |
|------------|------------|
MID1

for f in "${versions[@]}"; do
    unset VELERO_UPSTREAM_TAG VELERO_TAG_SHA
    . "$f"
    echo "| $VELERO_UPSTREAM_TAG | \`$VELERO_TAG_SHA\` |"
done

cat <<'MID2'

## Branch Coverage Matrix

Which repositories have downstream branches per OADP version:

| Repository | 1.3 | 1.4 | 1.5 | 1.6 | dev |
|------------|-----|-----|-----|-----|-----|
| openshift/velero | Y | Y | Y | Y | Y |
| openshift/restic | Y | Y | Y | Y | Y |
| migtools/kopia | Y | Y | Y | Y | Y |
| openshift/oadp-operator | Y | Y | Y | Y | Y |
| openshift/velero-plugin-for-aws | Y | Y | Y | Y | Y |
| openshift/velero-plugin-for-gcp | Y | Y | Y | Y | Y |
| openshift/velero-plugin-for-microsoft-azure | Y | Y | Y | Y | Y |
| openshift/openshift-velero-plugin | Y | Y | Y | Y | Y |
| openshift/oadp-must-gather | Y | Y | Y | Y | Y |
| migtools/kubevirt-velero-plugin | Y | Y | Y | Y | Y |
| openshift/velero-plugin-for-csi | Y | - | - | - | Y |
| openshift/velero-plugin-for-legacy-aws | - | Y | Y | Y | Y |
| migtools/oadp-non-admin | - | Y | Y | Y | Y |
| migtools/oadp-cli | - | Y | Y | Y | Y |
| openshift/hypershift-oadp-plugin | - | - | Y | Y | Y |
| migtools/filebrowser | - | - | - | Y | Y |
| migtools/oadp-vmdp | - | - | - | Y | Y |
| migtools/oadp-vm-file-restore | - | - | - | Y | Y |
| migtools/kubevirt-datamover-controller | - | - | - | Y | Y |
| migtools/kubevirt-datamover-plugin | - | - | - | Y | Y |
| migtools/udistribution | - | - | - | - | main |

## Wave Composition

Each wave groups repositories that can be rebased in parallel. Waves must be executed sequentially since later waves depend on earlier ones.

### OADP 1.3

| Wave | Repositories |
|------|-------------|
| 1 | kopia, restic |
| 2 | velero |
| 3 | kubevirt-velero-plugin, velero-plugin-for-csi, oadp-operator, velero-plugin-for-aws, velero-plugin-for-gcp, velero-plugin-for-microsoft-azure |
| 4 | openshift-velero-plugin |
| 5 | oadp-must-gather |

### OADP 1.4

| Wave | Repositories |
|------|-------------|
| 1 | kopia, restic |
| 2 | velero |
| 3 | kubevirt-velero-plugin, oadp-operator, velero-plugin-for-aws, velero-plugin-for-legacy-aws, velero-plugin-for-gcp, velero-plugin-for-microsoft-azure |
| 4 | oadp-non-admin, openshift-velero-plugin |
| 5 | oadp-must-gather, oadp-cli |

### OADP 1.5

| Wave | Repositories |
|------|-------------|
| 1 | kopia, restic |
| 2 | velero |
| 3 | kubevirt-velero-plugin, oadp-operator, velero-plugin-for-aws, velero-plugin-for-legacy-aws, velero-plugin-for-microsoft-azure, velero-plugin-for-gcp, hypershift-oadp-plugin |
| 4 | oadp-non-admin, openshift-velero-plugin |
| 5 | oadp-must-gather, oadp-cli |

### OADP 1.6

| Wave | Repositories |
|------|-------------|
| 1 | kopia, restic, filebrowser, oadp-vmdp |
| 2 | velero |
| 3 | kubevirt-velero-plugin, oadp-operator, velero-plugin-for-aws, velero-plugin-for-legacy-aws, velero-plugin-for-microsoft-azure, velero-plugin-for-gcp, hypershift-oadp-plugin |
| 4 | oadp-non-admin, openshift-velero-plugin, kubevirt-datamover-controller, oadp-vm-file-restore |
| 5 | oadp-must-gather, oadp-cli, kubevirt-datamover-plugin |

### oadp-dev

| Wave | Repositories |
|------|-------------|
| 1 | udistribution, kopia, restic, filebrowser, oadp-vmdp |
| 2 | velero |
| 3 | kubevirt-velero-plugin, velero-plugin-for-csi, oadp-operator, velero-plugin-for-aws, velero-plugin-for-legacy-aws, velero-plugin-for-microsoft-azure, velero-plugin-for-gcp, hypershift-oadp-plugin |
| 4 | oadp-non-admin, openshift-velero-plugin, kubevirt-datamover-controller, oadp-vm-file-restore |
| 5 | oadp-must-gather, oadp-cli, kubevirt-datamover-plugin |
MID2

} > "$tmp_output"

mv "$tmp_output" "$OUTPUT_FILE"
trap - EXIT

echo "Generated $OUTPUT_FILE"
