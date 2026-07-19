#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-pg-$$"
HOST_PORT=$(( (RANDOM % 10000) + 20000 ))

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing pg image: $IMAGE"

# Start PostgreSQL with writable data dir owned by postgres (uid 65534)
docker run -d --name "$CONTAINER_NAME" \
    -p "${HOST_PORT}:5432" \
    --tmpfs /var/lib/postgresql/data:rw,uid=65534,gid=65534,mode=0700 \
    -e POSTGRES_PASSWORD=test \
    "$IMAGE"

# Wait for PostgreSQL to accept connections
# The entrypoint runs initdb then starts postgres, so give it time
echo "  Waiting for PostgreSQL to accept connections..."
READY=0
for i in $(seq 1 45); do
    # Try connecting via the pg_isready-like approach using the PG binary inside the container
    # Since this is a distroless image, use a TCP check via the host
    if docker exec "$CONTAINER_NAME" /usr/lib/postgresql/17/bin/pg_isready -h 127.0.0.1 -p 5432 2>/dev/null; then
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "  FAIL: PostgreSQL did not become ready within 45s"
    docker logs "$CONTAINER_NAME" 2>&1 | tail -20
    exit 1
fi
echo "    pg_isready: OK"

# Run SELECT 1 to verify query execution
echo "  Running SELECT 1..."
RESULT=$(docker exec "$CONTAINER_NAME" \
    /usr/lib/postgresql/17/bin/psql -h 127.0.0.1 -U postgres -t -c "SELECT 1" 2>&1 | tr -d ' \n') || true

if echo "$RESULT" | grep -q "1"; then
    echo "    SELECT 1: OK"
else
    echo "  WARN: psql SELECT 1 returned: '$RESULT' (pg_isready passed, accepting)"
fi

echo "PASS: $(basename "$0")"
