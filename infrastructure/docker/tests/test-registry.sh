#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-registry-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing registry image: $IMAGE"

# The built-in config uses TLS, so we need to either use --insecure or
# override the config. Override entrypoint to serve without TLS for testing.
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:5000" \
    -e REGISTRY_HTTP_TLS_CERTIFICATE="" \
    -e REGISTRY_HTTP_TLS_KEY="" \
    "$IMAGE"

# Wait for /v2/ to return 200
echo "  Waiting for registry endpoint..."
READY=0
for i in $(seq 1 30); do
    # Try HTTPS first (built-in TLS cert), fall back to HTTP
    HTTP_CODE=$(curl -sk -o /dev/null -w "%{http_code}" "https://127.0.0.1:${HOST_PORT}/v2/" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/v2/" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: Registry did not respond on /v2/ within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    /v2/ returned 200: OK"

echo "PASS: $(basename "$0")"
