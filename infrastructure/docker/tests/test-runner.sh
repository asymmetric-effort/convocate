#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-runner-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing runner image: $IMAGE"

# Verify expected binaries exist in the runner image
BINARIES=("git" "curl" "docker" "node" "go" "kubectl" "helm" "ansible" "make" "jq")

for bin in "${BINARIES[@]}"; do
    echo "  Checking $bin..."
    if docker run --rm --entrypoint "" "$IMAGE" which "$bin" >/dev/null 2>&1; then
        echo "    $bin: OK"
    else
        echo "  FAIL: $bin not found in image"
        exit 1
    fi
done

echo "PASS: $(basename "$0")"
