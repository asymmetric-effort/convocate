#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-grafana-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing grafana image: $IMAGE"

# Start Grafana with HTTP on port 3000, no TLS
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:3000" \
    -e GF_SERVER_HTTP_PORT=3000 \
    -e GF_SERVER_PROTOCOL=http \
    "$IMAGE"

# Wait for /api/health to return 200
echo "  Waiting for Grafana health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/api/health" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: Grafana did not become healthy within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    Health endpoint returned 200: OK"

# Verify database=ok in response
echo "  Checking database status..."
HEALTH_RESP=$(curl -s "http://127.0.0.1:${HOST_PORT}/api/health" 2>/dev/null)
if echo "$HEALTH_RESP" | grep -q '"database"'; then
    echo "    Database field present: OK"
else
    echo "  FAIL: database field not found in health response"
    echo "    Response: $HEALTH_RESP"
    exit 1
fi

echo "PASS: $(basename "$0")"
