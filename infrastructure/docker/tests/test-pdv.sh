#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-pdv-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing pdv image: $IMAGE"

# Verify the test runner script exists at /tests/run-tests.sh
echo "  Checking /tests/run-tests.sh exists..."
if docker run --rm --entrypoint "" "$IMAGE" \
    /busybox/sh -c "test -f /tests/run-tests.sh && echo exists" 2>/dev/null | grep -q exists; then
    echo "    /tests/run-tests.sh: OK"
else
    echo "  FAIL: /tests/run-tests.sh not found in image"
    exit 1
fi

# Verify the script is executable
echo "  Checking /tests/run-tests.sh is executable..."
if docker run --rm --entrypoint "" "$IMAGE" \
    /busybox/sh -c "test -x /tests/run-tests.sh && echo executable" 2>/dev/null | grep -q executable; then
    echo "    Script is executable: OK"
else
    echo "  FAIL: /tests/run-tests.sh is not executable"
    exit 1
fi

echo "PASS: $(basename "$0")"
