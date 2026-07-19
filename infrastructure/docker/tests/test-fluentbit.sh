#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-fluentbit-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing fluentbit image: $IMAGE"

# Start Fluent Bit with a minimal dummy config (cpu input -> stdout output)
# This avoids the default config which may reference missing files
docker run -d --name "$CONTAINER_NAME" \
    --entrypoint /opt/fluent-bit/bin/fluent-bit \
    "$IMAGE" \
    -i cpu -o stdout -f 5

# Wait 5 seconds and verify it has not crashed
echo "  Waiting 5 seconds to verify stability..."
sleep 5

if docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "    Fluent Bit is running after 5s: OK"
else
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null)
    echo "  FAIL: Fluent Bit exited with code $EXIT_CODE within 5s"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi

echo "PASS: $(basename "$0")"
