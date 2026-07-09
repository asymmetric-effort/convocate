#!/bin/sh
# Synthetic health checks — runs every 5 minutes from K8s
# Reports pass/fail + latency to InfluxDB
set -e

INFLUXDB_URL="${INFLUXDB_URL:-https://influxdb.o11y.svc:8086}"
INFLUXDB_TOKEN="${INFLUXDB_TOKEN:-}"
INFLUXDB_ORG="${INFLUXDB_ORG:-convocate}"
INFLUXDB_BUCKET="${INFLUXDB_BUCKET:-logs}"
API_BASE="http://convocate-api.convocate.svc:8443"

RESULTS=""

# check_http: perform an HTTP GET/POST and record latency + pass/fail
# Usage: check_http <service_name> <method> <url> [post_data]
check_http() {
  svc="$1"
  method="$2"
  url="$3"
  post_data="${4:-}"
  now_ns="$(date +%s)000000000"
  start_ms="$(date +%s%N 2>/dev/null || echo 0)"

  # Build wget args
  wget_args="--no-check-certificate -q -O /dev/null -S --timeout=10"
  if [ "$method" = "POST" ]; then
    wget_args="$wget_args --post-data=${post_data}"
  fi

  # Execute the request and capture HTTP status
  status=0
  # wget -S prints headers to stderr; capture the HTTP status line
  http_code=$(wget $wget_args "$url" 2>&1 | grep "^ *HTTP/" | tail -1 | awk '{print $2}') || status=$?

  end_ms="$(date +%s%N 2>/dev/null || echo 0)"

  # Calculate latency in ms
  if [ "$start_ms" != "0" ] && [ "$end_ms" != "0" ]; then
    latency_ms=$(( (end_ms - start_ms) / 1000000 ))
  else
    latency_ms=0
  fi

  # Determine pass/fail
  if [ "$http_code" = "200" ]; then
    passed=1
    status_tag="pass"
  else
    passed=0
    status_tag="fail"
  fi

  RESULTS="${RESULTS}synthetic_check,service=${svc},status=${status_tag} latency_ms=${latency_ms}i,passed=${passed}i ${now_ns}
"
}

# check_api_service: check a sub-service from /api/v1/status JSON response
# Usage: check_api_service <service_name> <json_key>
check_api_service() {
  svc="$1"
  json_key="$2"
  now_ns="$(date +%s)000000000"
  start_ms="$(date +%s%N 2>/dev/null || echo 0)"

  # Fetch the status endpoint
  body=$(wget --no-check-certificate -q -O - --timeout=10 "${API_BASE}/api/v1/status" 2>/dev/null) || body=""

  end_ms="$(date +%s%N 2>/dev/null || echo 0)"

  if [ "$start_ms" != "0" ] && [ "$end_ms" != "0" ]; then
    latency_ms=$(( (end_ms - start_ms) / 1000000 ))
  else
    latency_ms=0
  fi

  # Parse the JSON for the service health status
  # Look for "json_key":{"status":"healthy"} pattern
  if echo "$body" | grep -q "\"${json_key}\"[^}]*\"healthy\""; then
    passed=1
    status_tag="pass"
  else
    passed=0
    status_tag="fail"
  fi

  RESULTS="${RESULTS}synthetic_check,service=${svc},status=${status_tag} latency_ms=${latency_ms}i,passed=${passed}i ${now_ns}
"
}

# check_auth_alive: verify auth endpoint is alive (401 = success)
check_auth_alive() {
  svc="api_auth"
  now_ns="$(date +%s)000000000"
  start_ms="$(date +%s%N 2>/dev/null || echo 0)"

  http_code=$(wget --no-check-certificate -q -O /dev/null -S --timeout=10 \
    --post-data='{"username":"","password":""}' \
    "${API_BASE}/api/v1/auth/login" 2>&1 | grep "^ *HTTP/" | tail -1 | awk '{print $2}') || true

  end_ms="$(date +%s%N 2>/dev/null || echo 0)"

  if [ "$start_ms" != "0" ] && [ "$end_ms" != "0" ]; then
    latency_ms=$(( (end_ms - start_ms) / 1000000 ))
  else
    latency_ms=0
  fi

  # Accept 200 or 401 (endpoint alive and rejecting bad creds)
  # Fail on 5xx or no response
  case "$http_code" in
    200|401) passed=1; status_tag="pass" ;;
    5*) passed=0; status_tag="fail" ;;
    *) if [ -n "$http_code" ] && [ "$http_code" -lt 500 ] 2>/dev/null; then
         passed=1; status_tag="pass"
       else
         passed=0; status_tag="fail"
       fi ;;
  esac

  RESULTS="${RESULTS}synthetic_check,service=${svc},status=${status_tag} latency_ms=${latency_ms}i,passed=${passed}i ${now_ns}
"
}

# check_node_metrics: verify /api/v1/nmgr/node returns JSON with items array
check_node_metrics() {
  svc="node_metrics"
  now_ns="$(date +%s)000000000"
  start_ms="$(date +%s%N 2>/dev/null || echo 0)"

  body=$(wget --no-check-certificate -q -O - --timeout=10 \
    --header="Authorization: Bearer mock-token" \
    "${API_BASE}/api/v1/nmgr/node" 2>/dev/null) || body=""

  end_ms="$(date +%s%N 2>/dev/null || echo 0)"

  if [ "$start_ms" != "0" ] && [ "$end_ms" != "0" ]; then
    latency_ms=$(( (end_ms - start_ms) / 1000000 ))
  else
    latency_ms=0
  fi

  # Check response contains an "items" array
  if echo "$body" | grep -q '"items"' && echo "$body" | grep -q '\['; then
    passed=1
    status_tag="pass"
  else
    passed=0
    status_tag="fail"
  fi

  RESULTS="${RESULTS}synthetic_check,service=${svc},status=${status_tag} latency_ms=${latency_ms}i,passed=${passed}i ${now_ns}
"
}

# check_agent_manager: verify /api/v1/amgr/agent responds 200 with auth
check_agent_manager() {
  svc="agent_manager"
  now_ns="$(date +%s)000000000"
  start_ms="$(date +%s%N 2>/dev/null || echo 0)"

  http_code=$(wget --no-check-certificate -q -O /dev/null -S --timeout=10 \
    --header="Authorization: Bearer mock-token" \
    "${API_BASE}/api/v1/amgr/agent" 2>&1 | grep "^ *HTTP/" | tail -1 | awk '{print $2}') || true

  end_ms="$(date +%s%N 2>/dev/null || echo 0)"

  if [ "$start_ms" != "0" ] && [ "$end_ms" != "0" ]; then
    latency_ms=$(( (end_ms - start_ms) / 1000000 ))
  else
    latency_ms=0
  fi

  if [ "$http_code" = "200" ]; then
    passed=1
    status_tag="pass"
  else
    passed=0
    status_tag="fail"
  fi

  RESULTS="${RESULTS}synthetic_check,service=${svc},status=${status_tag} latency_ms=${latency_ms}i,passed=${passed}i ${now_ns}
"
}

echo "=== Synthetic Health Checks ==="
echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# 1. API health
echo "Checking: api_health"
check_http "api_health" "GET" "${API_BASE}/api/v1/status"

# 2. API auth
echo "Checking: api_auth"
check_auth_alive

# 3. UI health
echo "Checking: ui_health"
check_http "ui_health" "GET" "http://convocate-ui.convocate.svc:8080/healthz"

# 4. PostgreSQL (via API status)
echo "Checking: postgresql"
check_api_service "postgresql" "postgresql"

# 5. Redis (via API status)
echo "Checking: redis"
check_api_service "redis" "redis"

# 6. OpenBao
echo "Checking: openbao"
check_http "openbao" "GET" "http://openbao.security.svc:8200/v1/sys/health"

# 7. InfluxDB
echo "Checking: influxdb"
check_http "influxdb" "GET" "https://influxdb.o11y.svc:8086/health"

# 8. Prometheus
echo "Checking: prometheus"
check_http "prometheus" "GET" "https://prometheus.o11y.svc:9090/-/ready"

# 9. Grafana
echo "Checking: grafana"
check_http "grafana" "GET" "http://grafana.o11y.svc:3000/api/health"

# 10. Ginger
echo "Checking: ginger"
check_http "ginger" "GET" "http://ginger.o11y.svc:16686/api/services"

# 11. MinIO
echo "Checking: minio"
check_http "minio" "GET" "http://minio.data-layer.svc:9000/minio/health/live"

# 12. Node metrics
echo "Checking: node_metrics"
check_node_metrics

# 13. Agent manager
echo "Checking: agent_manager"
check_agent_manager

# Print results for logging
echo ""
echo "=== Results ==="
printf '%s' "$RESULTS"

# Write to InfluxDB
echo ""
echo "Writing to InfluxDB..."
write_url="${INFLUXDB_URL}/api/v2/write?org=${INFLUXDB_ORG}&bucket=${INFLUXDB_BUCKET}&precision=ns"

response=$(wget --no-check-certificate -q -O /dev/null -S \
  --header="Authorization: Token ${INFLUXDB_TOKEN}" \
  --header="Content-Type: text/plain" \
  --post-data="$(printf '%s' "$RESULTS")" \
  "$write_url" 2>&1) || true

if echo "$response" | grep -q "HTTP/.*204\|HTTP/.*200"; then
  echo "OK: results written to InfluxDB"
else
  echo "WARN: InfluxDB write may have failed"
  echo "$response" | head -5
fi

echo ""
echo "Completed: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
