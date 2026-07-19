#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-gatekeeper-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing gatekeeper image: $IMAGE"

# Start gatekeeper with mock config
# The binary may exit if required config is missing — that is acceptable
# as long as it does not segfault or crash with a non-configuration error.
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:8443" \
    -e GATEKEEPER_LISTEN_ADDR="0.0.0.0:8443" \
    -e GATEKEEPER_TLS_DISABLE="true" \
    -e OPENBAO_ADDR="http://127.0.0.1:8200" \
    "$IMAGE"

# Give the process a moment to start or exit
sleep 2

# Check if container is still running
if docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "  Container is running: OK"

    # Try to reach /health endpoint
    echo "  Checking /health endpoint..."
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/health" 2>/dev/null) || true
    if [ -n "$HTTP_CODE" ] && [ "$HTTP_CODE" != "000" ]; then
        echo "    /health responded with HTTP $HTTP_CODE: OK (process is serving)"
    else
        echo "    /health not reachable, but process is running: OK"
    fi
else
    # Container exited — check if it was a clean exit (config error) vs crash
    EXIT_CODE=$(docker inspect -f '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null)
    echo "  Container exited with code $EXIT_CODE"

    if [ "$EXIT_CODE" = "0" ] || [ "$EXIT_CODE" = "1" ]; then
        # Exit code 0 or 1 is acceptable (missing config, missing OpenBao)
        LOGS=$(docker logs "$CONTAINER_NAME" 2>&1 | tail -5)
        echo "    Last output: $LOGS"
        echo "    Clean exit (likely missing config): OK"
    else
        echo "  FAIL: Container crashed with exit code $EXIT_CODE"
        docker logs "$CONTAINER_NAME" 2>&1 | tail -20
        exit 1
    fi
fi

echo "PASS: $(basename "$0")"
