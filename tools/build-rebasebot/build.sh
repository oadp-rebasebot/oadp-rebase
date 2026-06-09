#!/usr/bin/env bash
set -euo pipefail

REBASEBOT_REPO="https://github.com/openshift-eng/rebasebot.git"
IMAGE="quay.io/migtools/rebasebot:latest"
PLATFORMS="linux/amd64,linux/arm64"
CLONE_DIR="/tmp/rebasebot"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

DRY_RUN=false
for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=true ;;
        --help|-h)
            echo "Usage: $0 [--dry-run]"
            echo ""
            echo "Build and push a multi-arch rebasebot container image."
            echo ""
            echo "Options:"
            echo "  --dry-run   Build the image but don't push to quay.io"
            echo "  --help      Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $arg"
            exit 1
            ;;
    esac
done

echo "==> Cloning/updating rebasebot source..."
if [[ -d "$CLONE_DIR" ]]; then
    git -C "$CLONE_DIR" fetch origin
    git -C "$CLONE_DIR" checkout main
    git -C "$CLONE_DIR" reset --hard origin/main
else
    git clone "$REBASEBOT_REPO" "$CLONE_DIR"
fi

COMMIT=$(git -C "$CLONE_DIR" rev-parse --short HEAD)
echo "==> Building from commit: $COMMIT"

echo "==> Using Containerfile from $SCRIPT_DIR"
cp "$SCRIPT_DIR/Containerfile" "$CLONE_DIR/Containerfile"

echo "==> Creating manifest for $IMAGE..."
podman manifest rm "$IMAGE" 2>/dev/null || true
podman manifest create "$IMAGE"

echo "==> Building for platforms: $PLATFORMS"
podman build -f "$CLONE_DIR/Containerfile" --platform "$PLATFORMS" --manifest "$IMAGE" "$CLONE_DIR"

echo "==> Verifying manifest architectures..."
podman manifest inspect "$IMAGE" | jq -r '.manifests[] | "    \(.platform.os)/\(.platform.architecture)"'

if [[ "$DRY_RUN" == true ]]; then
    echo "==> Dry run — skipping push"
else
    echo "==> Pushing $IMAGE..."
    podman manifest push "$IMAGE" "docker://$IMAGE"
    echo "==> Done. Pushed $IMAGE (commit $COMMIT)"
fi
