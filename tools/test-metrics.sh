#!/usr/bin/env bash
#
# test-metrics.sh — Run tests across all modules, collect coverage and
# pass/fail counts, then push results to InfluxDB in line protocol format.
#
# Environment variables:
#   INFLUXDB_URL    — InfluxDB write endpoint (default: https://influxdb.o11y.svc:8086)
#   INFLUXDB_TOKEN  — Bearer token for InfluxDB writes (required for push)
#   INFLUXDB_ORG    — InfluxDB org (default: convocate)
#   INFLUXDB_BUCKET — InfluxDB bucket (default: logs)
#
# The script can run outside the cluster; if InfluxDB is unreachable or
# INFLUXDB_TOKEN is unset, metrics are printed to stdout but not pushed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

INFLUXDB_URL="${INFLUXDB_URL:-https://influxdb.o11y.svc:8086}"
INFLUXDB_TOKEN="${INFLUXDB_TOKEN:-}"
INFLUXDB_ORG="${INFLUXDB_ORG:-convocate}"
INFLUXDB_BUCKET="${INFLUXDB_BUCKET:-logs}"

TIMESTAMP_NS="$(date +%s)000000000"

LINES=()

# ── Helper: escape InfluxDB tag values ────────────────────
escape_tag() {
  local v="$1"
  v="${v// /\\ }"
  v="${v//,/\\,}"
  v="${v//=/\\=}"
  printf '%s' "$v"
}

# ── Go module tests ──────────────────────────────────────
run_go_module() {
  local module_name="$1"
  local module_dir="$2"

  echo "=== Testing Go module: ${module_name} (${module_dir}) ==="

  local tmpdir
  tmpdir="$(mktemp -d)"

  local passed=0
  local failed=0
  local total=0

  # Run tests with coverage, capture verbose output for pass/fail parsing
  if (cd "${module_dir}" && go test -coverprofile="${tmpdir}/coverage.out" -v ./... > "${tmpdir}/test.out" 2>&1); then
    echo "  Tests passed."
  else
    echo "  Some tests failed."
  fi

  # Parse pass/fail from verbose output
  if [[ -f "${tmpdir}/test.out" ]]; then
    passed=$(grep -c '^--- PASS:' "${tmpdir}/test.out" || true)
    failed=$(grep -c '^--- FAIL:' "${tmpdir}/test.out" || true)
    total=$((passed + failed))
    # If no individual test lines but "ok" lines exist, count packages
    if [[ "$total" -eq 0 ]]; then
      passed=$(grep -c '^ok' "${tmpdir}/test.out" || true)
      failed=$(grep -c '^FAIL' "${tmpdir}/test.out" || true)
      total=$((passed + failed))
    fi
  fi

  LINES+=("test_results,module=${module_name} passed=${passed}i,failed=${failed}i,total=${total}i ${TIMESTAMP_NS}")
  echo "  Results: passed=${passed} failed=${failed} total=${total}"

  # Parse per-package coverage from coverage.out
  if [[ -f "${tmpdir}/coverage.out" ]]; then
    while IFS= read -r line; do
      # Lines from "go tool cover -func" look like:
      # github.com/asymmetric-effort/convocate/internal/foo/bar.go:42:  FuncName  85.0%
      # The last line is: total:  (statements)  XX.X%
      local pct
      pct=$(echo "$line" | awk '{print $NF}' | tr -d '%')
      local func_path
      func_path=$(echo "$line" | awk '{print $1}')

      if echo "$func_path" | grep -q '^total:'; then
        # Overall module coverage
        local pkg="total"
        local tag
        tag="$(escape_tag "${pkg}")"
        LINES+=("test_coverage,module=${module_name},package=${tag} coverage=${pct} ${TIMESTAMP_NS}")
        echo "  Coverage total: ${pct}%"
      fi
    done < <(cd "${module_dir}" && go tool cover -func="${tmpdir}/coverage.out" 2>/dev/null || true)

    # Per-package coverage: parse coverage.out mode line then use go tool cover
    # We get per-package percentages by grouping
    while IFS= read -r line; do
      # go tool cover -func outputs per-function lines and a total line
      # We want per-package: extract unique package paths
      :
    done < /dev/null

    # Better approach: parse "go test -cover" output per package
    if [[ -f "${tmpdir}/test.out" ]]; then
      while IFS= read -r line; do
        # Lines like: ok  github.com/asymmetric-effort/convocate/internal/auth  0.003s  coverage: 100.0% of statements
        if echo "$line" | grep -q 'coverage:'; then
          local pkg_full
          pkg_full=$(echo "$line" | awk '{print $2}')
          # Extract short package name (last 2 path components)
          local pkg_short
          pkg_short=$(echo "$pkg_full" | rev | cut -d'/' -f1-2 | rev)
          local cov
          cov=$(echo "$line" | grep -o '[0-9]*\.[0-9]*%' | tr -d '%')
          if [[ -n "$cov" ]]; then
            local tag
            tag="$(escape_tag "${pkg_short}")"
            LINES+=("test_coverage,module=${module_name},package=${tag} coverage=${cov} ${TIMESTAMP_NS}")
            echo "  Coverage ${pkg_short}: ${cov}%"
          fi
        fi
      done < "${tmpdir}/test.out"
    fi
  fi

  rm -rf "${tmpdir}"
}

# Run all three Go modules
run_go_module "api" "${REPO_ROOT}/api"
run_go_module "metrics" "${REPO_ROOT}/metrics"
run_go_module "agent" "${REPO_ROOT}/agent"

# ── UI (Bun) tests ──────────────────────────────────────
echo "=== Testing UI (bun test) ==="
UI_DIR="${REPO_ROOT}/ui"
if [[ -d "${UI_DIR}" ]] && command -v bun &>/dev/null; then
  tmpfile="$(mktemp)"
  if (cd "${UI_DIR}" && bun test > "${tmpfile}" 2>&1); then
    echo "  UI tests passed."
  else
    echo "  Some UI tests failed."
  fi

  # Bun test output includes lines like:
  #  X pass
  #  Y fail
  #  or: "X pass, Y fail" on a summary line
  ui_passed=0
  ui_failed=0

  # Try parsing "N pass" and "N fail" from output
  if grep -qE '[0-9]+ pass' "${tmpfile}"; then
    ui_passed=$(grep -oE '[0-9]+ pass' "${tmpfile}" | tail -1 | awk '{print $1}')
  fi
  if grep -qE '[0-9]+ fail' "${tmpfile}"; then
    ui_failed=$(grep -oE '[0-9]+ fail' "${tmpfile}" | tail -1 | awk '{print $1}')
  fi
  ui_total=$((ui_passed + ui_failed))

  LINES+=("test_results,module=ui passed=${ui_passed}i,failed=${ui_failed}i,total=${ui_total}i ${TIMESTAMP_NS}")
  echo "  Results: passed=${ui_passed} failed=${ui_failed} total=${ui_total}"

  rm -f "${tmpfile}"
else
  echo "  Skipping: bun not available or src/ui/ not found."
fi

# ── Output collected metrics ─────────────────────────────
echo ""
echo "=== Collected InfluxDB Line Protocol ==="
for line in "${LINES[@]}"; do
  echo "$line"
done

# ── Push to InfluxDB ─────────────────────────────────────
if [[ -z "${INFLUXDB_TOKEN}" ]]; then
  echo ""
  echo "INFLUXDB_TOKEN not set — skipping InfluxDB push."
  echo "Set INFLUXDB_TOKEN to enable metrics reporting."
  exit 0
fi

BODY=""
for line in "${LINES[@]}"; do
  BODY="${BODY}${line}"$'\n'
done

echo ""
echo "Pushing ${#LINES[@]} metrics to ${INFLUXDB_URL} ..."

HTTP_CODE=$(curl -sk -o /dev/null -w '%{http_code}' \
  -X POST "${INFLUXDB_URL}/api/v2/write?org=${INFLUXDB_ORG}&bucket=${INFLUXDB_BUCKET}&precision=ns" \
  -H "Authorization: Token ${INFLUXDB_TOKEN}" \
  -H "Content-Type: text/plain" \
  --data-binary "${BODY}" \
  --connect-timeout 5 \
  --max-time 10 \
  2>/dev/null || echo "000")

if [[ "${HTTP_CODE}" == "204" || "${HTTP_CODE}" == "200" ]]; then
  echo "Successfully pushed metrics to InfluxDB (HTTP ${HTTP_CODE})."
elif [[ "${HTTP_CODE}" == "000" ]]; then
  echo "Could not connect to InfluxDB at ${INFLUXDB_URL} — skipping."
else
  echo "InfluxDB returned HTTP ${HTTP_CODE} — metrics may not have been stored."
fi
