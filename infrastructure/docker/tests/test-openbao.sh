#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-openbao-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing openbao image: $IMAGE"

# Start OpenBao with built-in config (tls_disable=true, file storage)
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:8200" \
    "$IMAGE"

# Wait for /v1/sys/health to respond
# 200 = initialized+unsealed, 429 = standby, 472 = recovery, 501 = not initialized, 503 = sealed
echo "  Waiting for OpenBao health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/v1/sys/health" 2>/dev/null) || true
    if [[ "$HTTP_CODE" =~ ^(200|429|472|501|503)$ ]]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: OpenBao did not become ready within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    Health endpoint responded with HTTP $HTTP_CODE: OK"

# Verify version endpoint
echo "  Checking version endpoint..."
VERSION_RESP=$(curl -s "http://127.0.0.1:${HOST_PORT}/v1/sys/health" 2>/dev/null)
if echo "$VERSION_RESP" | grep -q '"version"'; then
    echo "    Version endpoint: OK"
else
    echo "  FAIL: Version not found in health response"
    echo "    Response: $VERSION_RESP"
    exit 1
fi

echo "PASS: $(basename "$0")"
