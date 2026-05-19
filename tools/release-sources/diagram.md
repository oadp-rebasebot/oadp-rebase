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

- Branching note: many OADP code repos use `oadp-dev` as the default development branch (not `main`), while release flows use `oadp-1.x` style branches.
- If a branch-specific link 404s: try `oadp-dev` first, then the nearest release branch (`oadp-1.6`, `oadp-1.5`, etc.).
- CLI entrypoint: [`tools/release-sources/main.go`](./main.go)
- Source collection and compare logic:
  - [`FetchAll(...)`](./sources.go)
  - [`BuildUnion(...)`](./sources.go)
  - [`FindIssues(...)`](./sources.go)
  - render paths in [`render.go`](./render.go)
- External source systems used by `FetchAll(...)`:
  - Pyxis config: [`releng/pyxis-repo-configs/products/oadp/oadp.yaml`](https://gitlab.cee.redhat.com/releng/pyxis-repo-configs/-/blob/main/products/oadp/oadp.yaml)
  - OCP build-data images: [`openshift-eng/ocp-build-data/images`](https://github.com/openshift-eng/ocp-build-data/tree/oadp-1.5/images)
  - OCP build-data streams aliases: [`openshift-eng/ocp-build-data/streams.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/streams.yml)
  - OADP operator image references: [`openshift/oadp-operator/bundle/image-references`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/image-references)
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

Most step-by-step details for this flow (vars/files/envs/repos and exact code links) are consolidated in the walkthrough section below.

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

## End-to-end OADP release walk-through (intern-friendly, step-by-step)

```mermaid
flowchart TB
    S1["1) Choose release branch + source repos<br/>rebase-configs/*_<branch>.env.sh"]
    S2["2) Define per-image ART build wiring<br/>ocp-build-data/images/*.yml"]
    S3["3) Resolve Dockerfile FROM aliases<br/>ocp-build-data/streams.yml + ART tooling"]
    S4["4) Build component images from source repos<br/>Dockerfile/konflux.Dockerfile + code/submodules"]
    S5["5) Apply OADP operator bundle/CSV wiring<br/>group.yml + oadp-operator.yml + update-csv"]
    S6["6) Publish/maintain bundle/image-references"]
    S7["7) Build/publish File-Based Catalog (FBC)"]
    S8["8) Publish release visibility metadata<br/>Pyxis + advisories"]
    S9["9) Compare source-of-truth systems<br/>release-sources FetchAll/BuildUnion/FindIssues"]
    S10["10) Gate readiness/drift<br/>rebase-status checks"]
    S11["11) Special-case CVE checks<br/>oadp-must-gather ose-cli/oc path"]

    S1 --> S2 --> S3 --> S4 --> S5 --> S6 --> S7 --> S8 --> S9 --> S10
    S3 --> S11
    S4 --> S11
```

1. **Choose release branch + source repos**
   - branch-scoped mapping lives in [`rebase-configs/*_<branch>.env.sh`](../../rebase-configs/)
   - key variables: `SOURCE_UPSTREAM_REPO`, `DESTINATION_DOWNSTREAM_REPO`, `REBASE_REPO`
2. **Define per-image ART build wiring**
   - per-image config in [`openshift-eng/ocp-build-data/images`](https://github.com/openshift-eng/ocp-build-data/tree/oadp-1.5/images)
   - for operator specifically: [`openshift-eng/ocp-build-data/images/oadp-operator.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/images/oadp-operator.yml)
   - common fields to inspect/edit: `name`, `content.source.git.*`, `content.source.dockerfile`, `delivery_repo_names`, `update-csv.*`
3. **Resolve Dockerfile `FROM` aliases to concrete pullspecs**
   - alias source: [`openshift-eng/ocp-build-data/streams.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/streams.yml)
   - schema and examples for `from.stream` / builder streams:
     - [`openshift-eng/art-tools/ocp-build-data-validator/validator/json_schemas/image_config.base.schema.json`](https://github.com/openshift-eng/art-tools/blob/25f0a8d515ef029feaaa53162cfc2011d6913d56/ocp-build-data-validator/validator/json_schemas/image_config.base.schema.json)
     - [`openshift-eng/ocp-build-data/example/images/myutil-base.yml`](https://github.com/openshift-eng/ocp-build-data/blob/0a05a447bc6464f6c00a8a1948fba0b8a5953388/example/images/myutil-base.yml)
     - [`openshift-eng/ocp-build-data/example/images/template.yml`](https://github.com/openshift-eng/ocp-build-data/blob/0a05a447bc6464f6c00a8a1948fba0b8a5953388/example/images/template.yml)
   - ART tooling paths that read/resolve stream aliases:
     - [`openshift-eng/art-tools/doozer/doozerlib/image.py`](https://github.com/openshift-eng/art-tools/blob/25f0a8d515ef029feaaa53162cfc2011d6913d56/doozer/doozerlib/image.py)
     - [`openshift-eng/art-tools/doozer/doozerlib/backend/rebaser.py`](https://github.com/openshift-eng/art-tools/blob/25f0a8d515ef029feaaa53162cfc2011d6913d56/doozer/doozerlib/backend/rebaser.py)
     - [`openshift-eng/art-tools/doozer/doozerlib/backend/base_image_handler.py`](https://github.com/openshift-eng/art-tools/blob/25f0a8d515ef029feaaa53162cfc2011d6913d56/doozer/doozerlib/backend/base_image_handler.py)
     - [`openshift-eng/art-tools/pyartcd/pyartcd/pipelines/update_golang.py`](https://github.com/openshift-eng/art-tools/blob/25f0a8d515ef029feaaa53162cfc2011d6913d56/pyartcd/pyartcd/pipelines/update_golang.py)
4. **Build component images from source repos**
   - source repos (for example [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator/tree/oadp-1.5)) provide `konflux.Dockerfile`, `Dockerfile`, code, manifests, and optional submodules
   - CVE-impacting edit points commonly include:
     - builder/runtime base-image `FROM` lines in [`openshift/oadp-operator/konflux.Dockerfile`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/konflux.Dockerfile)
     - toolchain declarations in [`openshift/oadp-operator/go.mod`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/go.mod)
     - copied content via `COPY`/`ADD` in [`openshift/oadp-operator`](https://github.com/openshift/oadp-operator/tree/oadp-1.5)
     - submodule inputs in [`openshift/oadp-must-gather/.gitmodules`](https://github.com/openshift/oadp-must-gather/blob/oadp-1.5/.gitmodules)
     - helper hooks in this repo (for example [`rebasebot-hook-scripts/restic-submodule-and-commit_*.sh`](../../rebasebot-hook-scripts/))
5. **Apply OADP Operator bundle/CSV wiring**
   - catalog/version/env controls in [`openshift-eng/ocp-build-data/group.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/group.yml) (for example `GO_*`, `operator_image_ref_mode`, `FBC_DISABLE_CHANNEL_SKIPS`, `OCP_TARGET_VERSIONS`)
   - operator image + `update-csv` knobs in [`openshift-eng/ocp-build-data/images/oadp-operator.yml`](https://github.com/openshift-eng/ocp-build-data/blob/oadp-1.5/images/oadp-operator.yml)
   - resulting CSV wiring in [`openshift/oadp-operator/bundle/manifests/oadp-operator.clusterserviceversion.yaml`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/manifests/oadp-operator.clusterserviceversion.yaml) (`relatedImages`, `RELATED_IMAGE_*`)
6. **Publish/maintain `bundle/image-references` as release mapping**
   - file path: [`openshift/oadp-operator/bundle/image-references`](https://github.com/openshift/oadp-operator/blob/oadp-1.5/bundle/image-references)
   - treated as git-tracked source-of-truth in `openshift/oadp-operator` PRs (manual and automation updates both land as commits)
   - downstream coupling tests:
     - [`openshift/oadp-operator/tests/release/image_references.go`](https://github.com/openshift/oadp-operator/blob/oadp-dev/tests/release/image_references.go)
     - [`openshift/oadp-operator/tests/release/types.go`](https://github.com/openshift/oadp-operator/blob/oadp-dev/tests/release/types.go)
7. **Build/publish File-Based Catalog (FBC)**
   - ART/Konflux + `update-csv` output produce bundle/CSV inputs consumed by FBC publish flows for target OCP versions
8. **Publish release visibility metadata**
   - Pyxis product config: [`releng/pyxis-repo-configs/products/oadp/oadp.yaml`](https://gitlab.cee.redhat.com/releng/pyxis-repo-configs/-/blob/main/products/oadp/oadp.yaml)
   - Konflux advisory files: [`releng/konflux-release-data/advisories`](https://gitlab.cee.redhat.com/releng/konflux-release-data/-/tree/main/advisories)
9. **Compare source-of-truth systems in `release-sources`**
   - parse/fetch paths in [`FetchAll(...)`](./sources.go)
   - comparison paths in [`BuildUnion(...)`](./sources.go), [`FindIssues(...)`](./sources.go)
10. **Gate readiness/drift in `rebase-status`**
    - parse image-references in [`FetchImageReferences(...)`](../rebase-status/imageref.go)
    - cross-checks in [`crossRefImageArt(...)`](../rebase-status/checks.go), [`checkProductized(...)`](../rebase-status/checks.go)
11. **Special-case CVE checks for `oadp-must-gather`**
    - verify CLI image stage source in Dockerfile `FROM ...openshift-ose-cli...`
    - verify `COPY --from=ose-cli /usr/bin/oc /usr/bin/oc`
    - verify the matching `streams.yml` alias resolves to expected CLI/base pullspec

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

Detailed ownership/edit notes for this case are in walkthrough step **11**.

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

### C) Multi-image repo flow (different output fan-out)

Current known multi-output case from this codebase catalog: `migtools/oadp-vm-file-restore`.

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

- Primary edit/debug map is now consolidated in the walkthrough:
  - branch/repo/env inputs: steps **1-2**
  - base-image alias/debug paths (`streams.yml`, doozer, pyartcd): step **3**
  - Dockerfile, toolchain, COPY/ADD, submodule edit points: steps **4** and **11**
  - operator CSV/bundle/FBC controls: steps **5-7**
  - release publication and drift checks: steps **8-10**
