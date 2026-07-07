/**
 * InfluxDB PDV Reporter — sends test pass/fail and duration metrics
 * to InfluxDB after each test run for Grafana visualization.
 */

import type { Reporter, TestCase, TestResult, FullResult } from "@playwright/test/reporter";

// InfluxDB is cluster-internal; when running outside the cluster, metrics
// reporting is best-effort and will silently fail.
const INFLUXDB_URL = process.env.INFLUXDB_URL || "https://influxdb.o11y.svc:8086";
const INFLUXDB_TOKEN = process.env.INFLUXDB_TOKEN || "convocate-influx-token";
const INFLUXDB_ORG = "convocate";
const INFLUXDB_BUCKET = "logs";

// lgtm[js/disabling-certificate-validation] — PDV tests run against K8s services with self-signed TLS certificates
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

/** Escape a string for use as an InfluxDB line-protocol tag value.
 *  Tag values must escape spaces, commas, and equals signs. */
function escapeTagValue(s: string): string {
  return s.replace(/[, =]/g, (c) => "\\" + c);
}

class InfluxDBReporter implements Reporter {
  private lines: string[] = [];
  private runStart = 0;

  onBegin(): void {
    this.runStart = Date.now();
    this.lines = [];
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    const status = result.status; // "passed", "failed", "timedOut", "skipped"
    const passed = status === "passed" ? 1 : 0;
    const failed = status === "failed" || status === "timedOut" ? 1 : 0;
    const durationMs = result.duration;
    const project = escapeTagValue(test.parent?.project()?.name || "unknown");
    const suite = escapeTagValue(test.parent?.title || "unknown");
    const title = escapeTagValue(test.title);

    // InfluxDB line protocol: measurement,tag=value field=value timestamp
    const timestamp = Date.now() * 1000000; // nanoseconds
    this.lines.push(
      `pdv_test,project=${project},suite=${suite},test=${title},status=${escapeTagValue(status)} passed=${passed}i,failed=${failed}i,duration_ms=${durationMs}i ${timestamp}`
    );
  }

  async onEnd(result: FullResult): Promise<void> {
    const totalDuration = Date.now() - this.runStart;
    const timestamp = Date.now() * 1000000;

    // Add summary line
    this.lines.push(
      `pdv_run,result=${result.status} total_duration_ms=${totalDuration}i,passed=${this.lines.filter(l => l.includes("passed=1i")).length}i,failed=${this.lines.filter(l => l.includes("failed=1i")).length}i ${timestamp}`
    );

    // Send to InfluxDB
    const body = this.lines.join("\n");
    try {
      const res = await fetch(
        `${INFLUXDB_URL}/api/v2/write?org=${INFLUXDB_ORG}&bucket=${INFLUXDB_BUCKET}&precision=ns`,
        {
          method: "POST",
          headers: {
            Authorization: `Token ${INFLUXDB_TOKEN}`,
            "Content-Type": "text/plain",
          },
          body,
        }
      );
      if (res.ok) {
        console.log(`[influxdb-reporter] Sent ${this.lines.length} metrics to InfluxDB`);
      } else {
        console.log(`[influxdb-reporter] Failed to send metrics: ${res.status}`);
      }
    } catch (e: any) {
      console.log(`[influxdb-reporter] Error: ${e.message}`);
    }
  }
}

export default InfluxDBReporter;
