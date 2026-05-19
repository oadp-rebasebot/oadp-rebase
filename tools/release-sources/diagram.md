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

## Per-resulting-image diagrams (only when flow differs)

### A) Single-image repos (same/common path)

Examples: `velero`, `oadp-operator`, plugin repos, `kubevirt-datamover-controller`, `oadp-must-gather`.

```mermaid
flowchart LR
    A["source repo + konflux.Dockerfile"] --> B["ocp-build-data/images/<image>.yml"]
    C["ocp-build-data/streams.yml (FROM aliases)"] --> D["Konflux/ART build"]
    B --> D
    D --> E["1 resulting image"]
    E --> F["image-references + pyxis + advisories"]
```

### B) Multi-image repo flow (different output fan-out)

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
  - `rebase-configs/*_<branch>.env.sh`
  - keys: `SOURCE_UPSTREAM_REPO`, `DESTINATION_DOWNSTREAM_REPO`, `REBASE_REPO`
- **Go version / builder toolchain used by Konflux build**
  - repo `konflux.Dockerfile` builder `FROM ...:rhel_9_golang_* AS builder`
  - repo `go.mod` / toolchain declarations
- **Base image aliases used in Dockerfile `FROM` replacement**
  - `openshift-eng/ocp-build-data/blob/<branch>/streams.yml`
- **Per-image build wiring and source Dockerfile path**
  - `openshift-eng/ocp-build-data/blob/<branch>/images/*.yml`
  - fields commonly used: `name`, `content.source.git.*`, `content.source.dockerfile`, `delivery_repo_names`
- **COPYs / additional files included into image**
  - source repo `Dockerfile` / `konflux.Dockerfile` (`COPY`/`ADD` statements)
- **Submodules that can introduce vulnerable content**
  - source repo `.gitmodules`
  - plus this repo’s helper hooks where applicable (e.g. `rebasebot-hook-scripts/restic-submodule-and-commit_*.sh`)
- **Where resulting image refs are published/checked**
  - `openshift/oadp-operator/bundle/image-references`
  - `releng/pyxis-repo-configs/products/oadp/oadp.yaml`
  - `releng/konflux-release-data/.../oadp-advisory-{stage,prod}-*.yaml`
