<!-- Auto-generated from versions/*.env — do not edit manually. Run tools/generate-version-matrix.sh -->

# OADP Rebase Version Matrix

This document maps OADP versions to their upstream dependencies, tracks which repositories have downstream branches per version, and defines wave composition for the rebase process.

## Upstream Version Mapping

Each OADP release rebases against a specific set of upstream tags:

| OADP | Velero | Kopia | AWS Plugin | GCP Plugin | Azure Plugin | CSI Plugin | KubeVirt Plugin |
|------|--------|-------|------------|------------|--------------|------------|-----------------|
| 1.3 | v1.12.4 | v0.13.0-velero.1 | v1.8.2 | v1.8.2 | v1.8.2 | v0.6.3 | v0.6.2 |
| 1.4 | v1.14.0 | v0.17.0-velero.1 | v1.10.1 | v1.10.1 | v1.10.1 | n/a | v0.7.1 |
| 1.5 | v1.16.2 | v0.19.0-velero.1 | v1.12.2 | v1.12.2 | v1.12.2 | n/a | v0.8.0 |
| 1.6 | v1.18.2-rc.2 | v0.22.3-velero-patch | v1.14.1 | v1.14.1 | v1.14.1 | n/a | v0.9.0 |

The CSI plugin was merged into Velero core after v1.12, so OADP 1.4+ no longer has a separate CSI plugin repository.

## Velero Tag SHAs

Used by `verify-tag-sha` pre-rebase hooks to detect silent tag recreation (supply chain risk):

| Velero Tag | Commit SHA |
|------------|------------|
| v1.12.4 | `7d8417b2c58792422ed6dd4b4c0cf3848b0beb56` |
| v1.14.0 | `2fc6300f2239f250b40b0488c35feae59520f2d3` |
| v1.16.2 | `a60808256d36a77a42e1ebc160ee8117477a83fa` |
| v1.18.2-rc.2 | `c253c7fe37d78c9b7e55c68544f7c5b2608712d8` |

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
