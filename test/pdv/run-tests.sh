#!/bin/sh
# Convocate Post-Deployment Verification Tests
# Uses only busybox tools (wget, grep, sh) — no jq, no AVX-dependent binaries
set -e

API="${API_URL:-http://convocate-api.convocate.svc:8443}"
UI="${UI_URL:-http://convocate-ui.convocate.svc:8080}"
PASS=0
FAIL=0

log_pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
log_fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

fetch() { /busybox/wget -qO- --timeout=10 "$@" 2>/dev/null || true; }

echo "=== API Tests ==="

# Status endpoint (unauthenticated)
BODY=$(fetch "$API/api/v1/status")
if echo "$BODY" | /busybox/grep -q '"status":"healthy"'; then
    log_pass "GET /api/v1/status returns healthy"
else
    log_fail "GET /api/v1/status (got: $BODY)"
fi

# Login endpoint
BODY=$(fetch --post-data='{"username":"admin","password":"test","mfaToken":"123456"}' --header='Content-Type: application/json' "$API/api/v1/auth/login")
if echo "$BODY" | /busybox/grep -q '"accessToken"'; then
    log_pass "POST /api/v1/auth/login returns session"
else
    log_fail "POST /api/v1/auth/login (got: $BODY)"
fi

# Auth me endpoint
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/auth/me")
if echo "$BODY" | /busybox/grep -q '"username":"admin"'; then
    log_pass "GET /api/v1/auth/me returns admin principal"
else
    log_fail "GET /api/v1/auth/me (got: $BODY)"
fi

# Node Manager
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/nmgr/node")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/nmgr/node returns paginated nodes"
else
    log_fail "GET /api/v1/nmgr/node (got: $BODY)"
fi

# Agent Manager
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/amgr/agent")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/amgr/agent returns paginated agents"
else
    log_fail "GET /api/v1/amgr/agent (got: $BODY)"
fi

# Project Board
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/pb/board")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/pb/board returns paginated boards"
else
    log_fail "GET /api/v1/pb/board (got: $BODY)"
fi

# Access Control
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/ac/user")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/ac/user returns paginated users"
else
    log_fail "GET /api/v1/ac/user (got: $BODY)"
fi

# Repo Manager
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/repo/repo")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/repo/repo returns paginated repos"
else
    log_fail "GET /api/v1/repo/repo (got: $BODY)"
fi

# IDE Projects
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/ide/project")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/ide/project returns paginated projects"
else
    log_fail "GET /api/v1/ide/project (got: $BODY)"
fi

# Support Tickets
BODY=$(fetch --header='Authorization: Bearer mock-token' "$API/api/v1/sup/ticket")
if echo "$BODY" | /busybox/grep -q '"total"'; then
    log_pass "GET /api/v1/sup/ticket returns paginated tickets"
else
    log_fail "GET /api/v1/sup/ticket (got: $BODY)"
fi

echo ""
echo "=== UI Tests ==="

# UI health endpoint
BODY=$(fetch "$UI/healthz")
if echo "$BODY" | /busybox/grep -q '"status":"ok"'; then
    log_pass "GET /healthz returns ok"
else
    log_fail "GET /healthz (got: $BODY)"
fi

# UI serves index.html
BODY=$(fetch "$UI/")
if echo "$BODY" | /busybox/grep -q "Convocate"; then
    log_pass "GET / serves Convocate login page"
else
    log_fail "GET / does not contain Convocate"
fi

# Login form elements
if echo "$BODY" | /busybox/grep -q 'id="username"'; then
    log_pass "Login form contains username field"
else
    log_fail "Login form missing username field"
fi

# SPA fallback
BODY=$(fetch "$UI/some/unknown/path")
if echo "$BODY" | /busybox/grep -q "Convocate"; then
    log_pass "SPA fallback serves index.html for unknown paths"
else
    log_fail "SPA fallback not working"
fi

echo ""
echo "=== Results ==="
echo "Passed: $PASS  Failed: $FAIL  Total: $((PASS + FAIL))"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
