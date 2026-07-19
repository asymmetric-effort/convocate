#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-minio-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing minio image: $IMAGE"

# Start MinIO with required root credentials, writable data dir
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:9000" \
    --user 0:0 \
    -e MINIO_ROOT_USER=minioadmin \
    -e MINIO_ROOT_PASSWORD=minioadmin123 \
    "$IMAGE" \
    server /tmp/minio-data

# Wait for /minio/health/live to return 200
echo "  Waiting for MinIO health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/minio/health/live" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: MinIO did not become healthy within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    /minio/health/live returned 200: OK"

echo "PASS: $(basename "$0")"
