#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-redis-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing redis image: $IMAGE"

# Start Redis
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:6379" \
    "$IMAGE"

# Wait for port 6379 to accept connections
echo "  Waiting for Redis to accept connections..."
READY=0
for i in $(seq 1 30); do
    if docker exec "$CONTAINER_NAME" /usr/local/bin/redis-cli -p 6379 PING 2>/dev/null | grep -q PONG; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: Redis did not respond to PING within 30s"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    PING -> PONG: OK"

echo "PASS: $(basename "$0")"
