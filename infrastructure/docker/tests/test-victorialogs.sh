#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-victorialogs-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing victorialogs image: $IMAGE"

# Start VictoriaLogs
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:9428" \
    "$IMAGE"

# Wait for health endpoint
echo "  Waiting for VictoriaLogs health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/health" 2>/dev/null) || true
    if [[ "$HTTP_CODE" == "200" ]]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: VictoriaLogs did not become ready within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    Health endpoint responded with HTTP $HTTP_CODE: OK"

# Test log ingestion
echo "  Testing log ingestion..."
INGEST_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "http://127.0.0.1:${HOST_PORT}/insert/jsonline?_stream_fields=test&_msg_field=msg" \
    -d '{"msg":"test log entry","test":"e2e"}' 2>/dev/null) || true
if [[ "$INGEST_CODE" == "200" ]]; then
    echo "    Log ingestion: OK"
else
    echo "  FAIL: Log ingestion returned HTTP $INGEST_CODE"
    exit 1
fi

echo "PASS: $(basename "$0")"
