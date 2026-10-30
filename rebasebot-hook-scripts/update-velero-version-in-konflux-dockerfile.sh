#!/bin/bash
set -euo pipefail

# The runner loads versions/*.env and passes its Velero tag to hooks in both
# local CLI and container modes. No version is baked into this script.
VELERO_VERSION="${VELERO_UPSTREAM_TAG:?Missing VELERO_UPSTREAM_TAG from runner}"
DOCKERFILE="konflux.Dockerfile"

# Branch refs such as main must not be embedded as release versions.
if [[ ! "$VELERO_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[[:alnum:].-]+)?(\+[[:alnum:].-]+)?$ ]]; then
    echo "=== Skipping Velero version update (upstream ref is not a release tag: $VELERO_VERSION) ==="
    exit 0
fi

if [[ ! -f "$DOCKERFILE" ]]; then
    echo "=== Skipping $DOCKERFILE (file not found) ==="
    exit 0
fi

# Match only Velero's buildinfo field, including either upstream module path.
VERSION_FIELD='github\.com/(vmware-tanzu|velero-io)/velero/pkg/buildinfo\.Version='
if ! grep -qE "${VERSION_FIELD}[[:alnum:].+-]+" "$DOCKERFILE"; then
    echo "ERROR: Velero buildinfo.Version not found in $DOCKERFILE" >&2
    exit 1
fi

updated=$(mktemp)
trap 'rm -f "$updated"' EXIT
sed -E "s#(${VERSION_FIELD})[[:alnum:].+-]+#\1${VELERO_VERSION}-OADP#g" "$DOCKERFILE" > "$updated"

if cmp -s "$DOCKERFILE" "$updated"; then
    echo "=== No Velero version change needed in $DOCKERFILE ==="
    exit 0
fi

cat "$updated" > "$DOCKERFILE"
git add -- "$DOCKERFILE"
# Limit the commit to this file even if another hook left staged changes.
if [[ -n "${REBASEBOT_GIT_USERNAME:-}" && -n "${REBASEBOT_GIT_EMAIL:-}" ]]; then
    git commit --only --author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>" -q \
      -m "UPSTREAM: <drop>: update Velero version to ${VELERO_VERSION}" -- "$DOCKERFILE"
else
    git commit --only -q -m "UPSTREAM: <drop>: update Velero version to ${VELERO_VERSION}" -- "$DOCKERFILE"
fi
