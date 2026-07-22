#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-cloudflared-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing cloudflared image: $IMAGE"

# Verify cloudflared binary exists and reports its version
VERSION=$(docker run --rm --name "$CONTAINER_NAME" \
    --entrypoint /usr/local/bin/cloudflared \
    "$IMAGE" version 2>&1) || {
    echo "  FAIL: cloudflared version exited non-zero"
    echo "  Output: $VERSION"
    exit 1
}

if echo "$VERSION" | grep -q "cloudflared"; then
    echo "    cloudflared version: $VERSION"
else
    echo "  FAIL: unexpected version output: $VERSION"
    exit 1
fi

echo "PASS: $(basename "$0")"
