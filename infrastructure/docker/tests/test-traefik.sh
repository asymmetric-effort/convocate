#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-traefik-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing traefik image: $IMAGE"

# Verify traefik binary exists and reports its version
VERSION=$(docker run --rm --name "$CONTAINER_NAME" \
    --entrypoint /usr/local/bin/traefik \
    "$IMAGE" version 2>&1) || {
    echo "  FAIL: traefik version exited non-zero"
    echo "  Output: $VERSION"
    exit 1
}

if echo "$VERSION" | grep -q "traefik"; then
    echo "    traefik version: $VERSION"
else
    echo "  FAIL: unexpected version output: $VERSION"
    exit 1
fi

echo "PASS: $(basename "$0")"
