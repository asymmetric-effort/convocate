#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-influxdb-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing influxdb image: $IMAGE"

# Start InfluxDB with writable data dir
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:8086" \
    --user 0:0 \
    -e INFLUXD_BOLT_PATH=/tmp/influxdb/influxd.bolt \
    -e INFLUXD_ENGINE_PATH=/tmp/influxdb/engine \
    "$IMAGE"

# Wait for /health to return 200
echo "  Waiting for InfluxDB health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/health" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: InfluxDB did not become healthy within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    /health returned 200: OK"

echo "PASS: $(basename "$0")"
