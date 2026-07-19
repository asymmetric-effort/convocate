#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-prometheus-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing prometheus image: $IMAGE"

# Start Prometheus — the distroless image needs a minimal config
# Create a minimal prometheus.yml via a tmpfs or inline
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:9090" \
    --entrypoint "" \
    "$IMAGE" \
    /usr/local/bin/prometheus \
        --config.file=/dev/null \
        --storage.tsdb.path=/tmp/prometheus \
        --web.listen-address="0.0.0.0:9090"

# Wait for /-/healthy to return 200
echo "  Waiting for Prometheus health endpoint..."
READY=0
for i in $(seq 1 30); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${HOST_PORT}/-/healthy" 2>/dev/null) || true
    if [ "$HTTP_CODE" = "200" ]; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: Prometheus did not become healthy within 30s (last HTTP code: ${HTTP_CODE:-none})"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    /-/healthy returned 200: OK"

echo "PASS: $(basename "$0")"
