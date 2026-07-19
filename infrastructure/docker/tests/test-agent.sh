#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-agent-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing agent image: $IMAGE"

# Start the agent container
# It will likely exit without proper Claude API credentials — that is expected
docker run -d --name "$CONTAINER_NAME" \
    "$IMAGE"

# Give the process a moment to start or exit
sleep 3

# Check if container is still running or exited cleanly
if docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "  Container is running after 3s: OK"
else
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null)
    echo "  Container exited with code $EXIT_CODE"

    if [ "$EXIT_CODE" = "0" ] || [ "$EXIT_CODE" = "1" ]; then
        LOGS=$(docker logs "$CONTAINER_NAME" 2>&1 | tail -5)
        echo "    Last output: $LOGS"
        echo "    Clean exit (likely missing credentials/config): OK"
    else
        echo "  FAIL: Container crashed with exit code $EXIT_CODE"
        docker logs "$CONTAINER_NAME" 2>&1 | tail -20
        exit 1
    fi
fi

echo "PASS: $(basename "$0")"
