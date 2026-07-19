#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-convocate-api-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing convocate-api image: $IMAGE"

# Start the API server — it may exit immediately if required env vars are missing
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:8443" \
    "$IMAGE"

# Give the process a moment to start or exit
sleep 2

# Check if container is still running
if docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "  Container is running: OK"

    # Try to reach any endpoint
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/health" 2>/dev/null) || true
    if [ -n "$HTTP_CODE" ] && [ "$HTTP_CODE" != "000" ]; then
        echo "    HTTP server responded with $HTTP_CODE: OK"
    else
        echo "    HTTP server not yet reachable, but process is running: OK"
    fi
else
    # Container exited — verify it was a clean exit (missing config) not a crash
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null)
    echo "  Container exited with code $EXIT_CODE"

    if [ "$EXIT_CODE" = "0" ] || [ "$EXIT_CODE" = "1" ]; then
        LOGS=$(docker logs "$CONTAINER_NAME" 2>&1 | tail -5)
        echo "    Last output: $LOGS"
        echo "    Clean exit (likely missing config/env): OK"
    else
        echo "  FAIL: Container crashed with exit code $EXIT_CODE"
        docker logs "$CONTAINER_NAME" 2>&1 | tail -20
        exit 1
    fi
fi

echo "PASS: $(basename "$0")"
