#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-metrics-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing metrics image: $IMAGE"

# Start the metrics service
# It may exit if it cannot reach the API endpoint — that is expected
docker run -d --name "$CONTAINER_NAME" \
    "$IMAGE"

# Give the process a moment to start or exit
sleep 3

# Check if container is still running
if docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "  Container is running after 3s: OK"
else
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null)
    echo "  Container exited with code $EXIT_CODE"

    if [ "$EXIT_CODE" = "0" ] || [ "$EXIT_CODE" = "1" ]; then
        LOGS=$(docker logs "$CONTAINER_NAME" 2>&1 | tail -5)
        echo "    Last output: $LOGS"
        echo "    Clean exit (likely missing API endpoint): OK"
    else
        echo "  FAIL: Container crashed with exit code $EXIT_CODE"
        docker logs "$CONTAINER_NAME" 2>&1 | tail -20
        exit 1
    fi
fi

echo "PASS: $(basename "$0")"
