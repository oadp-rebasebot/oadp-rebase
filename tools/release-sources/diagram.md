# release-sources + build provenance diagrams

```mermaid
flowchart TD
    A["CLI: release-sources &lt;branch&gt;<br/>(main.go)"] --> B["FetchAll(gh, gl, branch)"]

    B --> C1["GitLab: pyxis-repo-configs<br/>products/oadp/oadp.yaml@main"]
    B --> C2["GitHub: openshift-eng/ocp-build-data<br/>images/*.yml@&lt;branch&gt;"]
    B --> C3["GitHub: openshift/oadp-operator<br/>bundle/image-references@&lt;branch&gt;"]
    B --> C4["GitLab: konflux-release-data<br/>oadp-advisory-stage-*&lt;suffix&gt;.yaml"]
    B --> C5["GitLab: konflux-release-data<br/>oadp-advisory-prod-*&lt;suffix&gt;.yaml"]

    C1 --> D["Sources{Pyxis, OBD, ImageRefs, Stage, Prod}<br/>+ metadata maps"]
    C2 --> D
    C3 --> D
    C4 --> D
    C5 --> D

    D --> E["BuildUnion(): all repos across sources<br/>(except known exceptions)"]
    D --> F["FindIssues(): missing-source checks<br/>+ OBD delivery_repo_names mismatch checks"]
    E --> G["Render output (table/json/markdown)"]
    F --> G

    subgraph Upstream_inputs["Origin/upstream inputs"]
      C2
      C3
    end

    subgraph Downstream_build_targets["Downstream/build signals"]
      C1
      C4
      C5
    end
```

## Hyperlinked reference map (step-by-step)

- CLI entrypoint: [`tools/release-sources/main.go`](./main.go)
- Source collection and compare logic:
  - [`FetchAll(...)`](./sources.go)
  - [`BuildUnion(...)`](./sources.go)
  - [`FindIssues(...)`](./sources.go)
  - render paths in [`render.go`](./render.go)
- External source systems used by `FetchAll(...)`:
  - Pyxis config: [`releng/pyxis-repo-configs/products/oadp/oadp.yaml`](https://gitlab.cee.redhat.com/releng/pyxis-repo-configs/-/blob/main/products/oadp/oadp.yaml)
  - OCP build-data images: [`openshift-eng/ocp-build-data/images/*.yml` (oadp-1.5)](https://github.com/openshift-eng/ocp-build-data/tree/oadp-1.5/images)
  - OCP build-data streams aliases: [`openshift-eng/ocp-build-data/streams.yml` (oadp-1.5)](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/streams.yml)
  - OADP operator image references: [`openshift/oadp-operator/bundle/image-references` (oadp-1.5)](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/image-references)
  - Konflux advisory data:
    - advisories repo path: [`releng/konflux-release-data/advisories`](https://gitlab.cee.redhat.com/releng/konflux-release-data/-/tree/main/advisories)
    - stage files: `oadp-advisory-stage-*.yaml`
    - prod files: `oadp-advisory-prod-*.yaml`

## Common build provenance flow (how source metadata becomes built images)

> `release-sources` compares source-of-truth metadata across systems.  
> It **does not build images itself** and **does not parse `streams.yml` directly**.  
> The build path below shows where `ocp-build-data/streams.yml` and `konflux.Dockerfile` are used by ART/Konflux.

```mermaid
flowchart LR
    U["Upstream/downstream source repo<br/>(contains Dockerfile/konflux.Dockerfile, go.mod, .gitmodules)"]
    R["rebase-configs/*.env.sh<br/>SOURCE_UPSTREAM_REPO<br/>DESTINATION_DOWNSTREAM_REPO<br/>REBASE_REPO"]
    A1["ocp-build-data/images/*.yml<br/>name, content.source.git.*, content.source.dockerfile,<br/>delivery_repo_names, distgit.component"]
    A2["ocp-build-data/streams.yml<br/>FROM alias → concrete base image pullspec"]
    K["Konflux/ART build<br/>reads Dockerfile FROM + aliases from streams.yml"]
    O["openshift/oadp-operator<br/>bundle/image-references<br/>(quay target refs)"]
    P["pyxis-repo-configs<br/>products/oadp/oadp.yaml"]
    S["konflux-release-data advisories<br/>stage/prod release payload entries"]
    F["Final container image(s)<br/>registry/quay + published metadata"]

    R --> U
    U --> A1
    A1 --> K
    A2 --> K
    K --> F
    F --> O
    F --> P
    F --> S
```

## OADP Operator catalog path (FBC / bundle / CSV / RELATED_IMAGES)

For OADP Operator, `ocp-build-data` has additional metadata beyond plain image build wiring:
- [`group.yml` (oadp-1.5)](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/group.yml) controls shared vars and catalog behavior (for example `GO_*`, `operator_image_ref_mode`, `FBC_DISABLE_CHANNEL_SKIPS`, `OCP_TARGET_VERSIONS`).
- [`images/oadp-operator.yml` (oadp-1.5)](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/images/oadp-operator.yml) defines `update-csv` inputs plus `delivery.bundle_delivery_repo_name` / `delivery_repo_names`.
- `update-csv` processing produces/updates bundle CSV content (including [`relatedImages` in CSV](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/manifests/oadp-operator.clusterserviceversion.yaml) and `RELATED_IMAGE_*` env references used by the operator deployment).
- Resulting operator/catalog payload files to inspect:
  - [`bundle/image-references` (oadp-1.5)](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/image-references)
  - [`bundle/manifests/oadp-operator.clusterserviceversion.yaml` (oadp-1.5)](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/manifests/oadp-operator.clusterserviceversion.yaml)

```mermaid
flowchart LR
    G["ocp-build-data/group.yml<br/>GO_* vars, operator_image_ref_mode,<br/>FBC_DISABLE_CHANNEL_SKIPS, OCP_TARGET_VERSIONS"]
    I["ocp-build-data/images/oadp-operator.yml<br/>content.source.*, update-csv.*, delivery.*"]
    S["openshift/oadp-operator source<br/>konflux.Dockerfile + manifests/ + bundle/"]
    B["ART/Konflux build + update-csv stage<br/>build operator image and refresh bundle/CSV metadata"]
    C["bundle image / CSV content<br/>contains relatedImages + RELATED_IMAGE_* driven refs"]
    F["File Based Catalog (FBC) content<br/>published for target OCP versions"]
    P["Published metadata systems<br/>image-references, pyxis, advisories"]

    G --> B
    I --> B
    S --> B
    B --> C
    C --> F
    C --> P
    F --> P
```

## Per-resulting-image diagrams (only when flow differs)

### A) Single-image repos (same/common path)

Examples: `velero`, `oadp-operator`, plugin repos, `kubevirt-datamover-controller`.

```mermaid
flowchart LR
    A["source repo + konflux.Dockerfile"] --> B["ocp-build-data/images/<image>.yml"]
    C["ocp-build-data/streams.yml (FROM aliases)"] --> D["Konflux/ART build"]
    B --> D
    D --> E["1 resulting image"]
    E --> F["image-references + pyxis + advisories"]
```

### B) `oadp-must-gather` special case (single output, multiple build sources)

`oadp-must-gather` is still one resulting image, but its build inputs are broader:
- main repo code,
- additional source trees (e.g. `velero`/`restic`/`kopia` via submodules or fetched sources depending on Dockerfile path),
- `oc` binary copied from OCP CLI image stage.

```mermaid
flowchart LR
    A["openshift/oadp-must-gather source"]
    B[".gitmodules or fetched external source trees<br/>(velero, restic, kopia; depends on selected Dockerfile path)"]
    C["Dockerfile or konflux.Dockerfile<br/>(selected by ocp-build-data/images/*.yml)"]
    D["ose-cli image stage<br/>(provides /usr/bin/oc)"]
    E["ART/Konflux build"]
    F["final oadp-must-gather image<br/>contains gather + helper binaries + oc"]

    A --> E
    B --> E
    C --> E
    D --> E
    E --> F
```

`oc`-related CVE ownership for `oadp-must-gather` is typically tied to:
- Dockerfile `FROM ...openshift-ose-cli...` stage tag/digest,
- Dockerfile `COPY --from=ose-cli /usr/bin/oc /usr/bin/oc`,
- and the `ocp-build-data/streams.yml` alias that resolves the CLI/base image pullspec used by ART.

### C) Multi-image repo flow (different output fan-out)

Current known multi-output case from this codebase catalog: `migtools/oadp-vm-file-restore` producing:
- `oadp-vm-file-restore`
- `oadp-vmfr-access`
- `oadp-vmfr-access-sshd`

```mermaid
flowchart LR
    A["single source repo + Dockerfile graph"] --> B["ocp-build-data/images/*.yml (same repo)"]
    C["streams.yml alias resolution"] --> D["Konflux/ART build"]
    B --> D
    D --> E1["image 1"]
    D --> E2["image 2"]
    D --> E3["image 3"]
    E1 --> F["published metadata systems"]
    E2 --> F
    E3 --> F
```

## Where to edit for CVE remediation (quick map)

- **Upstream/downstream source-of-truth repo mapping**
  - [`rebase-configs/*_<branch>.env.sh`](../../rebase-configs/)
  - keys: `SOURCE_UPSTREAM_REPO`, `DESTINATION_DOWNSTREAM_REPO`, `REBASE_REPO`
- **Go version / builder toolchain used by Konflux build**
  - repo [`konflux.Dockerfile`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/konflux.Dockerfile) builder `FROM ...:rhel_9_golang_* AS builder`
  - repo [`go.mod`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/go.mod) / toolchain declarations
- **Base image aliases used in Dockerfile `FROM` replacement**
  - [`openshift-eng/ocp-build-data/streams.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/streams.yml)
- **Per-image build wiring and source Dockerfile path**
  - [`openshift-eng/ocp-build-data/images/*.yml`](https://github.com/openshift-eng/ocp-build-data/tree/oadp-1.5/images)
  - fields commonly used: `name`, `content.source.git.*`, `content.source.dockerfile`, `delivery_repo_names`
- **Operator bundle/CSV/FBC wiring (OADP Operator path)**
  - [`ocp-build-data/group.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/group.yml) (catalog/version knobs such as `OCP_TARGET_VERSIONS`, `FBC_DISABLE_CHANNEL_SKIPS`)
  - [`ocp-build-data/images/oadp-operator.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/images/oadp-operator.yml) (`update-csv`, `bundle_delivery_repo_name`, `delivery_repo_names`)
  - [`oadp-operator CSV`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/manifests/oadp-operator.clusterserviceversion.yaml) (`relatedImages`, `RELATED_IMAGE_*` env usage)
- **COPYs / additional files included into image**
  - source repo [`Dockerfile` / `konflux.Dockerfile`](https://github.com/openshift/oadp-operator/tree/oadp-1.5) (`COPY`/`ADD` statements)
- **Submodules that can introduce vulnerable content**
  - source repo [`.gitmodules`](https://github.com/openshift/oadp-must-gather/blob/oadp-1.5/.gitmodules)
  - plus this repo’s helper hooks where applicable (e.g. [`rebasebot-hook-scripts/restic-submodule-and-commit_*.sh`](../../rebasebot-hook-scripts/))
- **Where resulting image refs are published/checked**
  - [`openshift/oadp-operator/bundle/image-references`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/image-references)
  - [`releng/pyxis-repo-configs/products/oadp/oadp.yaml`](https://gitlab.cee.redhat.com/releng/pyxis-repo-configs/-/blob/main/products/oadp/oadp.yaml)
  - [`releng/konflux-release-data` advisory files](https://gitlab.cee.redhat.com/releng/konflux-release-data/-/tree/main/advisories)
