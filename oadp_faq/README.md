# OADP frequently asked questions

Common questions and answers collected from recurring OADP support discussions.

<!-- BEGIN AUTO-MAINTAINED: forum-oadp -->
Last content update: 2026-10-30

## Installation and configuration

### Can I install multiple versions of OADP in the same cluster?

No. Different OADP releases ship different CustomResourceDefinitions, and only one OADP (and one Velero) installation works correctly in a cluster at a time. Install a single OADP version per cluster and upgrade it in place.

### Can OADP back up an entire cluster?

No. OADP backs up application resources that you select with namespaces or label selectors. It is not a platform backup: it does not back up or restore etcd, cluster operators, or the OpenShift platform itself. Scope each backup to the namespaces and resources you actually need so cluster-level resources are not pulled in.

### Can OADP be installed alongside other tooling that depends on a specific OADP version?

Only when the installed OADP version is the one that the other tooling expects. Tooling that pins an older OADP release cannot share a cluster with a newer OADP installation. Install the OADP version that the dependent tool requires, or keep that tool on a release that uses your installed OADP version.

### How can I check the details of the container images OADP deploys?

OADP container images are published per release in the Red Hat container catalog, under the OADP product listings. The catalog entries expose image contents, security metadata, source references, and included RPM packages. Releases from OADP 1.3 onward are built on RHEL 9 base images, and their image names carry the `-rhel9` suffix.

### How do I find the Velero version that OADP deployed?

Read the image tag on the `velero` deployment in the OADP namespace:

```
oc get deployment velero -n openshift-adp -o jsonpath='{.spec.template.spec.containers[0].image}'
```

With a default installation the tag reflects the Velero version that ships with that OADP release.

### How do I install and configure OADP on a ROSA cluster?

OADP is installed the same way as on other OpenShift clusters, with additional identity and storage prerequisites for the managed service. The supported procedure is documented in [backing up and restoring applications on ROSA](https://docs.openshift.com/rosa/rosa_backing_up_and_restoring_applications/backing-up-applications.html).

### How do I upgrade OADP?

Upgrade through Operator Lifecycle Manager and read the upgrade notes for the target release first, because some releases require changes to the DataProtectionApplication before or after the operator upgrade. See [OADP upgrade notes](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-upgrade-notes-1-6).

### Is OADP a complete data protection solution?

OADP provides the backup and restore APIs and the plugins that use them, and it is one component of a data protection strategy. Storage replication, immutability, retention policy, and application-consistent backup procedures remain your responsibility. See [OADP features and plugins](https://docs.openshift.com/container-platform/latest/backup_and_restore/application_backup_and_restore/oadp-features-plugins.html).

### What APIs does OADP provide?

OADP provides Backup, Restore, Schedule, BackupStorageLocation, and VolumeSnapshotLocation. These are Velero APIs: OADP does not add, remove, or modify the fields documented upstream, so the [Velero API reference](https://velero.io/docs/main/api-types/) applies.

### What data can OADP protect?

OADP backs up and restores Kubernetes and OpenShift API resources (the YAML that defines your applications), internal registry images (imagestreams), and persistent volume data through CSI snapshots, the CSI data mover, or file system backup.

### What does the OADP operator install?

The OADP operator installs Velero and the OpenShift plugins Velero uses for backup and restore, including the AWS, GCP, Azure, CSI, and KubeVirt plugins required by the configuration you enable.

### What is OADP?

OpenShift API for Data Protection (OADP) is an operator that adds backup and restore APIs to an OpenShift cluster, so application resources, images, and persistent volume data can be backed up to object storage and restored on demand or on a schedule. See the [OADP introduction](https://docs.openshift.com/container-platform/latest/backup_and_restore/application_backup_and_restore/oadp-intro.html).

### Which OpenShift Container Platform versions does an OADP release support?

Each OADP release supports a specific range of OpenShift Container Platform versions, and that mapping changes with every release. Confirm the supported combination in the release notes for the OADP version you intend to install, and in the OpenShift operator lifecycle information, before installing or upgrading. See [OADP release notes](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-release-notes).

### Why does the OADP version I installed not match my cluster version?

OADP releases are tied to OpenShift Container Platform versions. When the installed operator is newer than the release that supports your cluster version, installation and validation can fail. Install the OADP release that supports your current OpenShift version, or upgrade OpenShift to a version the installed OADP release supports. See [OADP operator version is higher than the supported cluster version](https://access.redhat.com/solutions/7138407).

## Backup storage and credentials

### Are S3 object lock, retention, and bucket versioning supported for backup data immutability?

OADP does not support S3 configuration for backup data immutability across providers, including S3 object lock, object retention, bucket versioning, and write-once-read-many (WORM) buckets. Do not rely on these features to protect backup data from deletion. See [backup data immutability support](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-support-backup-data-immutability_about-installing-oadp).

### Are virtual hosted-style bucket URLs supported for file system backup?

No. File system backup does not work with virtual hosted-style bucket URLs, where the bucket name is part of the host name instead of the path. Use path-style addressing, such as `s3ForcePathStyle: "true"` for S3-compatible providers, when file system backup is enabled.

### Do I need to set checksumAlgorithm on an S3-compatible backup storage location?

For S3-compatible storage such as NooBaa or ODF, a backup can fail when `checksumAlgorithm` is unset. From OADP 1.4 onward, OADP adds an empty `checksumAlgorithm` value to backup storage locations that it creates from the DataProtectionApplication, so DPA-managed locations work without you setting it. Locations created by other means are not adjusted.

### Does OADP support short-lived credentials such as STS or workload identity federation?

Yes. OADP supports token-based, short-lived authentication with ROSA and AWS STS, Google Cloud workload identity federation (WIF), and Azure security token service (STS). Volume snapshot locations are not supported with GCP WIF or Azure STS, and imagestream backup is not supported with Azure STS.

### How do I use S3-compatible storage that has no region?

Some S3-compatible implementations do not use regions but still require a region value. Put any valid AWS region string, such as `us-east-1`, in the `region` field and keep the real endpoint in `s3Url`:

```yaml
backupLocations:
  - velero:
      provider: aws
      default: true
      config:
        profile: default
        region: us-east-1
        s3Url: https://<s3-endpoint>
        s3ForcePathStyle: "true"
      credential:
        key: cloud
        name: cloud-credentials
      objectStorage:
        bucket: <bucket>
        prefix: <prefix>
```

### What S3-compatible object storage is supported, and which plugin should I use?

Supported providers and their known limitations are listed in [S3-compatible backup storage providers](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-s3-compatible-backup-storage-providers_about-installing-oadp). OADP ships both an `aws` plugin, built on the newer AWS SDK, and a `legacy-aws` plugin for providers whose behavior differs between SDK versions. Try `aws` first and switch to `legacy-aws` only when the provider is incompatible with it.

### Where do ODF encryption secrets need to be for encrypted volume backup and restore?

When ODF volumes are encrypted with a key management service, the secrets used for that encryption must also be present in the OADP namespace. Without them, backup and restore of the encrypted volumes fail.

## Backups and schedules

### Can I turn off backup of internal registry images?

Yes. Set `backupImages: false` in the DataProtectionApplication spec to disable image (imagestream) backup and restore. Do this when imagestream handling causes backup or restore errors and you do not need registry image content in your backups.

```yaml
spec:
  backupImages: false
```

### Does a backup disappear as soon as its TTL expires?

No. Expiration is applied by the garbage collector, which reconciles on an hourly cycle, so backup data can remain in object storage for up to roughly an hour after the TTL passes.

### How do I exclude build pod volumes so a BuildConfig backup does not partially fail?

Build pods have usually completed by the time a backup runs, so their volumes cannot be read and the backup ends PartiallyFailed. Exclude the build volumes with the `backup.velero.io/backup-volumes-excludes` annotation, selecting the pods by their build label:

```
oc annotate -n <namespace> $(oc get pods -n <namespace> -o name -l openshift.io/build.name) \
  backup.velero.io/backup-volumes-excludes=buildworkdir,container-storage-root,container-storage-run,build-blob-cache
```

### How do I set a backup to expire?

Set a time-to-live when you create the backup with `--ttl <duration>`, for example `--ttl 720h`. When Velero sees an expired backup it removes the backup resource, its files in object storage, its persistent volume snapshots, and any restores created from it. See [set a backup to expire](https://velero.io/docs/main/how-velero-works/#set-a-backup-to-expire).

### Where can I find example Backup and Restore manifests?

The OADP documentation includes worked backup and restore procedures with sample manifests; the application backup guide is the usual starting point. See [backing up and restoring applications](https://docs.openshift.com/container-platform/latest/backup_and_restore/application_backup_and_restore/backing_up_and_restoring/backing-up-applications.html).

## Restore and migration

### Can OADP restore routes with the destination cluster's base domain?

Yes, for generated routes. A route whose `spec.host` was left empty for OpenShift to generate carries `openshift.io/host.generated: "true"`, and its host is stripped on backup so the destination cluster generates one from its own base domain.

For routes with an explicitly set host, OADP has no mechanism to rewrite the host to the destination cluster's base domain; the original host is restored. Edits made to `.spec.host` of a generated route are also lost, and adding the generated annotation manually to a route can lead to unexpected restore behavior.

### How do I avoid data access problems caused by changed namespace UID and GID ranges after restore?

Every OpenShift namespace is assigned a UID range, a supplemental group range, and SELinux MCS labels, stored in the `openshift.io/sa.scc.uid-range`, `openshift.io/sa.scc.supplemental-groups`, and `openshift.io/sa.scc.mcs` annotations. If a namespace already carries those annotations, OpenShift keeps them instead of assigning new ones, so restored workloads can end up with a container UID that no longer matches the owner of the restored files.

Common causes and mitigations:

- A pre-existing namespace on the destination cluster with different annotations: remove the namespace before restoring, or align file ownership after restore.
- A label selector backup that did not include the namespace object: add the matching label to the namespace so it is backed up with its annotations.

See [resolving overlapping UID ranges in OpenShift namespaces after migration](https://access.redhat.com/articles/6844071) for the procedure that repairs file ownership.

### How do I back up from one cluster and restore into another?

Both clusters need backup storage locations with matching names and object storage paths, and they must share the same object storage credentials. Let OADP create the destination namespace, and delete existing PVCs that the restore must recreate. With the restic uploader, also remove the workload that mounts those volumes before restoring into a namespace where the volumes already exist.

See [backing up data from one cluster and restoring to another cluster](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#backing-up-data-one-cluster-restoring-another-cluster).

### Should backup storage be read-only during a disaster recovery restore?

Yes, setting `accessMode: ReadOnly` on the backup storage location while restoring prevents additions and deletions in the backup storage during the restore. Return the location to read-write when normal backups resume.

```yaml
spec:
  backupLocations:
    - velero:
        accessMode: ReadOnly
```

### Why do restored PVCs stay unbound when the storage class uses WaitForFirstConsumer binding?

With node-agent restore enabled, PVCs on a storage class that binds `WaitForFirstConsumer` can fail to bind when node affinity is configured, which leaves the restore waiting on PVCs that never become bound. Set `ignoreDelayBinding` in the DataProtectionApplication so the node agent ignores the binding delay:

```yaml
spec:
  configuration:
    nodeAgent:
      enable: true
      restorePVC:
        ignoreDelayBinding: true
```

### Why does a restore fail when a GitOps controller manages the same resources?

The `app.kubernetes.io/instance` label used by GitOps controllers such as Argo CD can collide with the resources OADP manages during a restore, and the restore then fails. Set the controller's resource tracking method to `annotation+label` or `annotation` so it stops claiming ownership through labels. If the restore still fails, disable the controller for the duration of the restore and re-enable it afterwards.

## CSI snapshots

### Does OADP support CSI snapshots?

Yes. CSI snapshots require the CSI plugin to be enabled in the DataProtectionApplication and the volume driver to support snapshots. See [enabling CSI in the DPA](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-enabling-csi-dpa_installing-oadp-aws).

### How do I back up only PVC data without the owning pods?

Include only `persistentvolumeclaims` in the backup and scope it to the namespaces you need. The PVCs are captured through CSI snapshots without the pods that mount them.

```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: <backup-name>
  namespace: openshift-adp
spec:
  includedNamespaces:
    - <namespace>
  includedResources:
    - persistentvolumeclaims
  storageLocation: <storage-location>
  ttl: 720h
```

The CSI plugin must be enabled and the [CSI backup prerequisites](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-backing-up-pvs-csi-doc) must be met.

### Why can't Azure Files PVCs be restored from snapshots?

The Azure Files CSI driver supports taking snapshots but not restoring from them, so PVCs on Azure Files cannot be recovered from a CSI snapshot. Use file system backup, or the CSI data mover, for Azure Files volumes instead. See [OADP or Velero restore of an Azure Files CSI PVC creates an empty file share](https://access.redhat.com/solutions/7011280).

### Why does a VolumeSnapshotContent end up in a failed state?

During a backup one VolumeSnapshotContent is created per volume, and some of them briefly report errors such as `Failed to check and update snapshot content` together with `the object has been modified; please apply your changes to the latest version and try again`. This is an update conflict in the snapshot controller and normally clears as OpenShift reconciles the objects.

If a VolumeSnapshotContent stays failed and the backup does not progress, it can be deleted and the controller recreates it. Confirm afterwards that the resulting backup restores with the expected data.

## Data movement and file system backup

### How can I see how much data an incremental backup actually moved?

The snapshot history in the Kopia repository shows both the cumulative size of a snapshot and what that snapshot added. Compare `new-data` and `new-files` across snapshots of the same volume: the first snapshot carries a large `new-data` value, and later snapshots carry only the changed bytes.

### How do I inspect the Kopia repository OADP writes to?

Velero's Kopia repositories are ordinary Kopia repositories in object storage, so the Kopia CLI can connect to them with the bucket, prefix, and repository password that Velero uses. Treat the repository as read-only while inspecting it, and do not modify snapshots that backups depend on.

### How should node agent nodeSelector labels be configured?

The node agent `nodeSelector` uses Kubernetes semantics: every label in the selector must exist on a node for the node agent pod to run there. Listing several roles is not an either-or choice, so a selector containing both `node-role.kubernetes.io/infra` and `node-role.kubernetes.io/worker` only matches nodes that carry both labels.

Label the nodes that should run the node agent with one dedicated label and select on that label:

```
oc label node <node> node-role.kubernetes.io/nodeagent=""
```

```yaml
configuration:
  nodeAgent:
    enable: true
    podConfig:
      nodeSelector:
        node-role.kubernetes.io/nodeagent: ""
```

### What does an unbound PVC and pending pod after a data mover restore mean?

This is expected while the data mover is still running. The application PVC keeps a `spec.selector` that binds it to the restored persistent volume, so it stays pending, and the CSI driver may report that a claim selector is not supported, until the DataDownload finishes. Do not remove the selector from the PVC; diagnose the DataDownload instead.

### What does "at least one source file could not be read" mean in a file system backup?

The backup scanned the volume and then found a file missing when it tried to read it, which happens when files churn during the backup. The backup reports the error and finishes with warnings, and the missing file is simply not in the snapshot.

Reduce the effect by quiescing the workload during backup, excluding the churning volume, or using a CSI snapshot instead of file system backup for that volume.

### Why are data mover pods not placed where the node agent runs in OADP 1.5?

In OADP 1.5, `loadAffinity` restricts where node agent pods run but is not applied to the data mover pods that process DataUploads and DataDownloads. If such a pod lands on a node without a node agent, the upload or download is never processed. Upgrade to OADP 1.6 or later, or stop using `loadAffinity` as a workaround.

### Why do ODF and Ceph volumes need extra configuration for the data mover?

Data mover PVCs on ODF and Ceph storage can need the volume settings described in [configuring backup and restore PVCs for data mover](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#configuring-backup-restore-pvc-datamover), including file system group and SELinux relabel handling, before the data mover pods can write to them.

### Why does a data mover restore of a large, mostly empty disk appear to stall near completion?

The DataDownload finishes reading from object storage quickly and then spends a long time writing, so progress stops near 100% and the restore can time out. This is common with virtual machine disks that are large but mostly empty: Kopia writes the empty regions as zeros through the node's page cache, and the final flush to the storage backend is slow and silent.

Set `writeSparseFiles: true` on the Restore so Kopia skips zero regions:

```yaml
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: <restore-name>
  namespace: openshift-adp
spec:
  backupName: <backup-name>
  uploaderConfig:
    writeSparseFiles: true
```

The setting applies to `volumeMode: Filesystem` PVCs only and must be set explicitly on each restore. Block-mode volumes still write the full stream, so their restore time is bound by backend write throughput.

### Why is ODF capacity not reclaimed after a backup or restore?

ODF does not always reclaim space that backups and restores have freed, so capacity stays higher than the data in use suggests. Reclaim the space with the ODF reclaim space tooling, such as a `ReclaimSpaceCronJob`, instead of assuming the backup left data behind.

## OpenShift Virtualization

### How do I avoid virt-freeze failures during VM backups?

Backups of virtual machines can fail in the virt-freeze step, most often on Windows guests where the freeze command intermittently fails, and retrying the backup is the usual recovery. To keep a virtual machine from being frozen, prevent the automatic backup hooks with the skip annotation:

```yaml
metadata:
  annotations:
    kubevirt.io/skip-backup-hooks: "true"
```

Newer OADP releases contain fixes for these freeze issues, so upgrade if you are on an affected version. See [OADP or Cohesity backup freezes a VM during backup](https://access.redhat.com/solutions/7137659).

### Should I regenerate VM identity after restoring a VM?

When a restored virtual machine runs alongside the original, regenerate its machine UUID and MAC addresses so the two instances do not present duplicate identities or collide on network addresses. A restore that replaces the only copy of a virtual machine does not need new identity values. See [labels to apply on a virtual machine restore definition](https://access.redhat.com/solutions/7141128).

### Why does backing up a Windows VM finish as PartiallyFailed?

A Windows virtual machine backup can end PartiallyFailed when the guest agent does not complete the freeze or thaw step in time. Re-run the backup, and if it keeps failing, back the virtual machine up with the freeze hooks skipped. See [OADP backup of Windows VMs ends with PartiallyFailed status](https://access.redhat.com/solutions/7101204).

### Why is Ceph capacity higher than expected after restoring VMs?

Restoring virtual machines into Ceph-backed storage can consume more capacity than the source cluster reported for the same virtual machines, because of how restored disks are provisioned. Plan for the difference; see [restoring a single virtual machine](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-restore-single-vm_installing-oadp-kubevirt).

## Multi-tenant backup

### Can non-cluster-admin users create their own backups and restores?

Yes. From OADP 1.5 onward, OADP supports self-service backup and restore, so users without cluster administrator rights can back up and restore their own namespaces. See [OADP self-service](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-self-service).

### Can OADP be installed into multiple namespaces so different teams manage their own instance?

Yes. Multiple OADP deployments can run in one cluster, each in its own namespace, so different administrators manage their own instance. Every deployment must be the same OADP version, and each needs its own credentials and backup storage location configuration. Each deployment still has cluster-wide access by default.

## Troubleshooting

### How do I collect OADP diagnostics with must-gather?

Run must-gather with the OADP image that matches your OADP release:

```
oc adm must-gather --image=registry.redhat.io/oadp/oadp-mustgather-rhel9:<oadp-version> -- /usr/bin/gather --skip-tls
```

The `--skip-tls` argument is needed when the cluster does not trust the certificates that gather uses. See [running must-gather](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/backup_and_restore/oadp-application-backup-and-restore#oadp-running-must-gather_using-the-must-gather-tool).

### Why do node agent pods crash on managed cloud platforms that relocate kubelet data?

Some managed platforms store kubelet data outside `/var/lib/kubelet`, so the host path the node agent assumes does not exist and the node agent pods crash. OADP detects clusters running on such platforms and adjusts the host paths automatically. If you override the paths yourself, verify the real kubelet pods and plugins directories on a node first, then set `FS_PV_HOSTPATH` and `PLUGINS_HOSTPATH` to match.

### Why does an OADP upgrade fail with a podvolumerestores schema error?

An install plan can fail while validating existing `podvolumerestores.velero.io` objects against the new CRD schema when the cluster already contains restores created with an older OADP. Finished PodVolumeRestores are safe to delete, since new ones are created for each restore. Delete them and retry the upgrade.

### Why does the Velero CLI fail with x509 certificate errors?

`backup describe`, `backup download`, `backup logs`, `restore describe`, and `restore logs` read directly from object storage, so they verify the storage endpoint certificate against the client's trust store. Certificate settings in the DataProtectionApplication do not change that behavior for a CLI running outside the cluster.

Pass the storage CA bundle explicitly with `--cacert`, or use `--insecure-skip-tls-verify` for a quick check, which is not recommended for production. When running the CLI through the in-cluster deployment, copy the CA bundle into the pod first:

```
oc get dpa -n openshift-adp <dpa-name> -o jsonpath='{.spec.backupLocations[0].velero.objectStorage.caCert}' \
  | base64 -d | oc exec -i deploy/velero -n openshift-adp -c velero -- bash -c 'cat > /tmp/ca-bundle.pem'
```

Then run `velero backup describe <backup-name> --details --cacert /tmp/ca-bundle.pem`.

<!-- END AUTO-MAINTAINED: forum-oadp -->
