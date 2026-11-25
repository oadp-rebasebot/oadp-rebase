#!/usr/bin/env bash
# Tests for rebasebot-hook-scripts/normalize-dockerfiles-and-commit.sh: patch
# versions are stripped, and Go builders older than go.mod's go directive are
# raised to it (upstream golang:X.Y and rhel_9_golang_X.Y only; any other
# builder form behind is warned about, never rewritten).
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
hook="$here/../../rebasebot-hook-scripts/normalize-dockerfiles-and-commit.sh"
fail=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

check() { # name expected actual
  if [ "$2" = "$3" ]; then echo "  PASS  $1"; else echo "  FAIL  $1: expected '$2', got '$3'"; fail=1; fi
}

repo() { # dir go-directive (empty: no go.mod)
  mkdir -p "$1" && git -C "$1" init -q && git -C "$1" config user.email t@example.com && git -C "$1" config user.name t
  [ -n "$2" ] && printf 'module example.com/m\n\ngo %s\n' "$2" > "$1/go.mod"
}

commit_all() { git -C "$1" add -A && git -C "$1" commit -q -m base; }
run_hook() { (cd "$1" && bash "$hook" 2>&1); }
commits() { git -C "$1" rev-list --count HEAD; }

# go.mod moved to 1.26: every raisable builder follows, nothing else changes.
r="$tmp/raise"; repo "$r" 1.26.0
cat > "$r/Containerfile.download" <<'DF'
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
DF
cat > "$r/konflux.Dockerfile" <<'DF'
FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_golang_1.25 AS builder
DF
printf 'FROM golang:1.25.7-bookworm AS b\n' > "$r/Dockerfile"
printf 'FROM golang:1.27 AS b\n' > "$r/Dockerfile.newer"
printf 'FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.20 AS b\n' > "$r/Dockerfile.ci"
printf "docker_build('golang:1.25', '.')\n" > "$r/Tiltfile"
printf 'FROM docker.io/library/golang:1.25-alpine AS b\n' > "$r/Dockerfile.library"
printf 'FROM golang:1.20-alpine3.18 AS b\n' > "$r/Dockerfile.ospinned"
printf 'FROM quay.io/acme/custom-golang:1.25 AS b\n' > "$r/Dockerfile.custom"
printf 'FROM golang:1.25@sha256:0123abcd AS b\n' > "$r/Dockerfile.digest"
printf 'FROM registry.redhat.io/openshift/golang-builder:golang-builder-v1.25-rhel9 AS b\n' > "$r/Dockerfile.rhgb"
commit_all "$r"
out=$(run_hook "$r")
check "upstream golang raised" "FROM --platform=\$BUILDPLATFORM golang:1.26 AS builder" "$(head -n1 "$r/Containerfile.download")"
check "non-Go image untouched" "FROM registry.access.redhat.com/ubi9/ubi-minimal:latest" "$(sed -n 2p "$r/Containerfile.download")"
check "brew builder raised" "FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_golang_1.26 AS builder" "$(cat "$r/konflux.Dockerfile")"
check "patch stripped, then raised" "FROM golang:1.26-bookworm AS b" "$(cat "$r/Dockerfile")"
check "newer builder never lowered" "FROM golang:1.27 AS b" "$(cat "$r/Dockerfile.newer")"
check "other builder form not rewritten" "FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.20 AS b" "$(cat "$r/Dockerfile.ci")"
check "other builder form warned" "1" "$(grep -c '::warning file=Dockerfile.ci::' <<<"$out")"
check "non-FROM reference untouched" "docker_build('golang:1.25', '.')" "$(cat "$r/Tiltfile")"
check "docker.io/library prefix and -alpine raised" "FROM docker.io/library/golang:1.26-alpine AS b" "$(cat "$r/Dockerfile.library")"
# Never written: the raised tag may not be published, or would not change Go.
for f in ospinned custom digest rhgb; do
  check "$f builder not rewritten" "$(git -C "$r" show "HEAD~1:Dockerfile.$f")" "$(cat "$r/Dockerfile.$f")"
  check "$f builder warned" "1" "$(grep -c "::warning file=Dockerfile.$f::" <<<"$out")"
done
check "raised builders not warned" "0" "$(grep -cE '::warning file=(Containerfile.download|konflux.Dockerfile|Dockerfile|Dockerfile.library)::' <<<"$out")"
check "one commit added" "2" "$(commits "$r")"

# Builders already current: nothing to commit.
c="$tmp/current"; repo "$c" 1.25
printf 'FROM golang:1.25 AS b\n' > "$c/Dockerfile"
commit_all "$c"; run_hook "$c" > /dev/null
check "current builder, no commit" "1" "$(commits "$c")"

# No go.mod: patch stripping still runs, nothing is raised.
n="$tmp/nogomod"; repo "$n" ""
printf 'FROM golang:1.24.3 AS b\n' > "$n/Dockerfile"
commit_all "$n"; run_hook "$n" > /dev/null
check "no go.mod, only stripped" "FROM golang:1.24 AS b" "$(cat "$n/Dockerfile")"

# 1.9 < 1.26 by version, not by text.
v="$tmp/version"; repo "$v" 1.26
printf 'FROM golang:1.9 AS b\n' > "$v/Dockerfile"
commit_all "$v"; run_hook "$v" > /dev/null
check "version order" "FROM golang:1.26 AS b" "$(cat "$v/Dockerfile")"

[ "$fail" -eq 0 ] && echo "All normalize-dockerfiles tests passed" || exit 1
