#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Return the exit status of the last command in the pipe that failed

# Normalize Go builder image versions in Dockerfiles / Containerfiles /
# Tiltfiles.
#
# Problem: Dockerfiles that pin a Go patch version (e.g. golang:1.25.7)
# miss future CVE fixes because the tag never moves.  Using just the
# minor version (golang:1.25) always resolves to the latest patch.
#
# What this script does:
#   1. Finds all Dockerfile*, Containerfile*, *.Dockerfile (e.g.
#      konflux.Dockerfile) and Tiltfile* in the repository.  Tiltfiles
#      embed a dockerfile (container_build) whose FROM lines reference
#      the same "golang:" images, e.g. velero's tilt-helper image.
#   2. In FROM lines that reference the upstream "golang:" image,
#      strips the patch version: golang:1.25.7-bookworm -> golang:1.25-bookworm
#   3. Leaves non-golang images untouched (konveyor/builder, brew.registry,
#      registry.ci.openshift.org, ubi, distroless, etc.).
#   4. Raises a Go builder that is older than go.mod's `go` directive to
#      that minor version (raise_go_builders, below).
#   5. Commits if anything changed.
#
# This is safe because:
#   - "golang:1.25" is a floating tag that always points to the latest
#     1.25.x release, which includes security fixes.
#   - Downstream builds (konflux.Dockerfile, Dockerfile.ubi) already use
#     Red Hat builder images that don't pin patch versions.
#
# Step 4 exists because a rebase can raise go.mod's `go` directive (an
# upstream or dependency bump) while the branch's builders stay behind, and
# the build then fails with "go.mod requires go >= 1.26.0 (running go
# 1.25.x; GOTOOLCHAIN=local)" (migtools/oadp-non-admin#399,
# migtools/oadp-cli#289). A newer toolchain still builds the older module,
# so raising is always safe. Only tags known to exist for every Go minor
# are rewritten: upstream `golang:X.Y` and `golang:X.Y-{bookworm,trixie,alpine}`
# (optionally docker.io/library/), and the Red Hat
# `{brew.registry.redhat.io,registry-proxy.engineering.redhat.com}/rh-osbs/
# openshift-golang-builder:rhel_9_golang_X.Y`, each matched as the whole
# image reference. Any other Go builder still behind -- another repository,
# an OS-pinned suffix such as -alpine3.18, a digest pin (golang:X.Y@sha256:..),
# golang-builder:golang-builder-vX.Y-rhel9, ocp/builder:rhel-9-golang-X.Y-.. --
# is reported with a ::warning:: and left for a person, as is the ci-operator
# build_root in openshift/release, which lives outside the repo.

stage_and_commit(){
    if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
        author_flag=()
    else
        author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
    fi

    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Normalize Go version in Dockerfiles"
    fi
}

normalize_dockerfiles() {
    echo "Normalizing Go builder versions in Dockerfiles/Tiltfiles"

    local changed=0

    # Find all Dockerfile* and Containerfile* (any depth)
    while IFS= read -r -d '' dockerfile; do
        # Only process files that have a golang: FROM instruction with a
        # patch version.  A FROM instruction starts a line, or follows the
        # opening quote of an embedded dockerfile string (Tiltfile's
        # dockerfile_contents = """FROM ...).  Match patterns like:
        #   FROM golang:1.25.7
        #   FROM golang:1.25.7-bookworm
        #   FROM golang:1.23.6-bullseye
        #   FROM golang:1.25.0
        # But NOT:
        #   FROM golang:1.25
        #   FROM golang:1.25-bookworm  (already correct)
        #   brew.registry.redhat.io/...  (not "golang:" prefix)
        #   registry.ci.openshift.org/... (not "golang:" prefix)
        #   quay.io/konveyor/builder:... (not "golang:" prefix)
        #   docker_build('.../golang:1.25.7', ...)  (not a FROM instruction)
        if grep -qE '(^|["[:space:]])[Ff][Rr][Oo][Mm][[:space:]].*\bgolang:[0-9]+\.[0-9]+\.[0-9]+' "$dockerfile"; then
            echo "=== Normalizing $dockerfile ==="

            # Strip patch version from golang: image references, only on
            # FROM instruction lines, so image names referenced elsewhere
            # (e.g. Tiltfile docker_build calls) are left untouched.
            # golang:1.25.7-bookworm  ->  golang:1.25-bookworm
            # golang:1.25.8           ->  golang:1.25
            # golang:1.23.6-bullseye  ->  golang:1.23-bullseye
            # golang:1.25.0           ->  golang:1.25
            sed -i -E '/(^|["[:space:]])[Ff][Rr][Oo][Mm][[:space:]]/s/(golang:[0-9]+\.[0-9]+)\.[0-9]+/\1/g' "$dockerfile"

            changed=1
        else
            echo "=== Skipping $dockerfile (no golang patch version found) ==="
        fi
    done < <(find . \( -name 'Dockerfile*' -o -name 'Containerfile*' -o -name '*.Dockerfile' -o -name 'Tiltfile*' \) -not -path '*/vendor/*' -print0)

    if [ "$changed" -eq 0 ]; then
        echo "=== No Dockerfiles needed normalization ==="
    fi

    raise_go_builders

    stage_and_commit
}

raise_go_builders() {
    if [ ! -f go.mod ]; then
        echo "=== No go.mod at the repository root; Go builders left as they are ==="
        return 0
    fi
    local want
    want=$(awk '$1 == "go" { print $2; exit }' go.mod | grep -oE '^[0-9]+\.[0-9]+' || true)
    if [ -z "$want" ]; then
        echo "=== go.mod has no go directive; Go builders left as they are ==="
        return 0
    fi
    echo "Raising Go builders older than go.mod's go ${want}"

    local files=()
    mapfile -d '' files < <(find . \( -name 'Dockerfile*' -o -name 'Containerfile*' -o -name '*.Dockerfile' -o -name 'Tiltfile*' \) -not -path '*/vendor/*' -print0)
    [ "${#files[@]}" -gt 0 ] || return 0

    # Each image reference on a FROM instruction is matched whole. Only
    # references whose raised tag is published for every Go minor are
    # rewritten; anything else still behind -- another repository, an
    # OS-pinned suffix such as -alpine3.18, a digest pin, another builder
    # form -- is reported with a ::warning:: and left unchanged.
    python3 - "$want" "${files[@]}" <<'PY'
import re
import sys

want = sys.argv[1]
want_v = tuple(int(x) for x in want.split("."))
from_kw = re.compile(r'(^|["\s])[Ff][Rr][Oo][Mm]\s')
token = re.compile(r'[^\s"\']+')
raisable = (
    # Upstream image; these suffixes are published for every minor.
    re.compile(r'^((?:docker\.io/)?(?:library/)?golang:)(\d+)\.(\d+)((?:-(?:bookworm|trixie|alpine))?)$'),
    # Red Hat builder.
    re.compile(r'^((?:brew\.registry\.redhat\.io|registry-proxy\.engineering\.redhat\.com)'
               r'/rh-osbs/openshift-golang-builder:rhel_9_golang_)(\d+)\.(\d+)()$'),
)
go_version = re.compile(r'golang(?:-builder-v|[-_:]v?)(\d+)\.(\d+)', re.IGNORECASE)

for path in sys.argv[2:]:
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    changed = False
    for i, line in enumerate(lines):
        kw = from_kw.search(line)
        if not kw:
            continue
        out, pos = [], kw.end()
        out.append(line[:pos])
        for m in token.finditer(line, pos):
            ref = m.group(0)
            out.append(line[pos:m.start()])
            pos = m.end()
            for pattern in raisable:
                r = pattern.match(ref)
                if r and (int(r.group(2)), int(r.group(3))) < want_v:
                    new = f"{r.group(1)}{want}{r.group(4)}"
                    print(f"=== {path}: {ref} -> {new} ===")
                    ref, changed = new, True
                    break
            else:
                v = go_version.search(ref)
                if v and (int(v.group(1)), int(v.group(2))) < want_v:
                    print(f"::warning file={path.removeprefix('./')}::Go builder {ref} is older than "
                          f"go.mod's go {want} and is not raised automatically; update it by hand")
            out.append(ref)
        out.append(line[pos:])
        lines[i] = "".join(out)
    if changed:
        with open(path, "w", encoding="utf-8") as f:
            f.write("\n".join(lines))
PY
}

normalize_dockerfiles
