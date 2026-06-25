#!/usr/bin/env bash
# E2E tests for infrastructure Docker containers (Redis, PostgreSQL, OpenBao).
# Usage: ./test/e2e/docker_test.sh
# Requires: docker, docker compose
set -euo pipefail

COMPOSE_FILE="docker-compose.yml"
PROJECT="convocate-e2e"
PASS=0
FAIL=0
TESTS=()

cleanup() {
    echo "--- Cleaning up ---"
    docker compose -p "$PROJECT" -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

log_pass() {
    PASS=$((PASS + 1))
    TESTS+=("PASS: $1")
    echo "  PASS: $1"
}

log_fail() {
    FAIL=$((FAIL + 1))
    TESTS+=("FAIL: $1")
    echo "  FAIL: $1"
}

wait_for_healthy() {
    local service="$1"
    local max_wait="${2:-60}"
    local elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        local health
        health=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
            ps --format json "$service" 2>/dev/null \
            | head -1 | grep -o '"Health":"[^"]*"' | cut -d'"' -f4) || true
        if [ "$health" = "healthy" ]; then
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1
}

# ─────────────────────────────────────────────────────
echo "=== Building containers ==="
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" build --quiet

echo ""
echo "=== Starting containers ==="
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d

# ─────────────────────────────────────────────────────
echo ""
echo "=== Redis Tests ==="

if wait_for_healthy redis 60; then
    log_pass "redis container is healthy"
else
    log_fail "redis container did not become healthy within 60s"
fi

# Test: redis accepts connections and responds to PING
REDIS_PONG=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T redis /usr/local/bin/redis-cli PING 2>/dev/null) || true
if [ "$REDIS_PONG" = "PONG" ]; then
    log_pass "redis responds to PING"
else
    log_fail "redis did not respond PONG (got: $REDIS_PONG)"
fi

# Test: redis SET/GET round-trip
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T redis /usr/local/bin/redis-cli SET convocate_test "hello" > /dev/null 2>&1 || true
REDIS_GET=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T redis /usr/local/bin/redis-cli GET convocate_test 2>/dev/null) || true
if [ "$REDIS_GET" = "hello" ]; then
    log_pass "redis SET/GET round-trip"
else
    log_fail "redis SET/GET failed (got: $REDIS_GET)"
fi

# Test: redis INFO returns server info
REDIS_INFO=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T redis /usr/local/bin/redis-cli INFO server 2>/dev/null | head -5) || true
if echo "$REDIS_INFO" | grep -q "redis_version"; then
    log_pass "redis INFO returns server version"
else
    log_fail "redis INFO did not contain version"
fi

# ─────────────────────────────────────────────────────
echo ""
echo "=== PostgreSQL Tests ==="

if wait_for_healthy postgresql 90; then
    log_pass "postgresql container is healthy"
else
    log_fail "postgresql container did not become healthy within 90s"
fi

# Test: psql can connect and run a query
PG_RESULT=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps \
    postgresql /usr/local/bin/pg/psql -U postgres -t -c "SELECT 1" 2>/dev/null) || true
if echo "$PG_RESULT" | grep -q "1"; then
    log_pass "postgresql SELECT 1 succeeds"
else
    log_fail "postgresql SELECT 1 failed (got: $PG_RESULT)"
fi

# Test: create database
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps \
    postgresql /usr/local/bin/pg/psql -U postgres -c "CREATE DATABASE convocate_test" > /dev/null 2>&1 || true
PG_DB=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps \
    postgresql /usr/local/bin/pg/psql -U postgres -t -c \
    "SELECT datname FROM pg_database WHERE datname = 'convocate_test'" 2>/dev/null) || true
if echo "$PG_DB" | grep -q "convocate_test"; then
    log_pass "postgresql CREATE DATABASE succeeds"
else
    log_fail "postgresql CREATE DATABASE failed"
fi

# Test: create table, insert, select
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps \
    postgresql /usr/local/bin/pg/psql -U postgres -d convocate_test -c \
    "CREATE TABLE test_table (id SERIAL PRIMARY KEY, val TEXT NOT NULL);
     INSERT INTO test_table (val) VALUES ('e2e_check');" > /dev/null 2>&1 || true
PG_VAL=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps \
    postgresql /usr/local/bin/pg/psql -U postgres -d convocate_test -t -c \
    "SELECT val FROM test_table WHERE id = 1" 2>/dev/null) || true
if echo "$PG_VAL" | grep -q "e2e_check"; then
    log_pass "postgresql table create/insert/select round-trip"
else
    log_fail "postgresql table round-trip failed (got: $PG_VAL)"
fi

# ─────────────────────────────────────────────────────
echo ""
echo "=== OpenBao Tests ==="

# OpenBao starts in sealed state; wait for the process to be listening
sleep 5
BAO_UP=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e BAO_ADDR=http://127.0.0.1:8200 \
    openbao /usr/local/bin/bao status -format=json 2>/dev/null) || true
if echo "$BAO_UP" | grep -q '"initialized"'; then
    log_pass "openbao is running and responding to status"
else
    log_fail "openbao is not responding to status"
fi

# Test: initialize OpenBao
BAO_INIT=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
    exec -T -e BAO_ADDR=http://127.0.0.1:8200 \
    openbao /usr/local/bin/bao operator init \
    -key-shares=1 -key-threshold=1 -format=json 2>/dev/null) || true
UNSEAL_KEY=$(echo "$BAO_INIT" | grep -o '"unseal_keys_b64":\["[^"]*"' | cut -d'"' -f4) || true
ROOT_TOKEN=$(echo "$BAO_INIT" | grep -o '"root_token":"[^"]*"' | cut -d'"' -f4) || true
if [ -n "$UNSEAL_KEY" ] && [ -n "$ROOT_TOKEN" ]; then
    log_pass "openbao operator init succeeds"
else
    log_fail "openbao operator init failed"
fi

# Test: unseal OpenBao
if [ -n "$UNSEAL_KEY" ]; then
    docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
        exec -T -e BAO_ADDR=http://127.0.0.1:8200 \
        openbao /usr/local/bin/bao operator unseal "$UNSEAL_KEY" > /dev/null 2>&1 || true
    BAO_SEALED=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
        exec -T -e BAO_ADDR=http://127.0.0.1:8200 \
        openbao /usr/local/bin/bao status -format=json 2>/dev/null \
        | grep -o '"sealed":false') || true
    if [ "$BAO_SEALED" = '"sealed":false' ]; then
        log_pass "openbao unseal succeeds"
    else
        log_fail "openbao unseal failed"
    fi
fi

# Test: write and read a secret
if [ -n "$ROOT_TOKEN" ]; then
    docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
        exec -T -e BAO_ADDR=http://127.0.0.1:8200 -e BAO_TOKEN="$ROOT_TOKEN" \
        openbao /usr/local/bin/bao secrets enable -path=secret kv-v2 > /dev/null 2>&1 || true
    docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
        exec -T -e BAO_ADDR=http://127.0.0.1:8200 -e BAO_TOKEN="$ROOT_TOKEN" \
        openbao /usr/local/bin/bao kv put -mount=secret convocate/test key=e2e_value > /dev/null 2>&1 || true
    BAO_SECRET=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" \
        exec -T -e BAO_ADDR=http://127.0.0.1:8200 -e BAO_TOKEN="$ROOT_TOKEN" \
        openbao /usr/local/bin/bao kv get -mount=secret -format=json convocate/test 2>/dev/null) || true
    if echo "$BAO_SECRET" | grep -q "e2e_value"; then
        log_pass "openbao secret write/read round-trip"
    else
        log_fail "openbao secret round-trip failed"
    fi
fi

# ─────────────────────────────────────────────────────
echo ""
echo "=== Results ==="
for t in "${TESTS[@]}"; do
    echo "  $t"
done
echo ""
echo "Passed: $PASS  Failed: $FAIL  Total: $((PASS + FAIL))"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
