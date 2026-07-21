// Smoke tests for Kubernetes cluster-b (production).
// Lightweight checks: nodes Ready, basic pod scheduling works,
// logging pipeline operational.

import { test, expect } from "@playwright/test";
import { execSync } from "child_process";

const KUBECONFIG = process.env.KUBECONFIG || "/tmp/kubeconfig";
const CLUSTER_NAME = process.env.CLUSTER_NAME || "cluster-b";
const VICTORIALOGS_HOST = "192.168.3.167"; // logs-b (production)

function kubectl(cmd: string): string {
  return execSync(`kubectl --kubeconfig=${KUBECONFIG} ${cmd}`, {
    encoding: "utf-8",
    timeout: 120000,
  }).trim();
}

test.describe(`${CLUSTER_NAME} Smoke`, () => {
  test("kubectl can reach the API server", () => {
    const version = kubectl("version --short 2>/dev/null || kubectl version");
    expect(version).toContain("Server Version");
  });

  test("all 6 nodes are in Ready state", () => {
    const nodes = kubectl("get nodes --no-headers");
    const lines = nodes.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBe(6);
    for (const line of lines) {
      expect(line).toContain("Ready");
      expect(line).not.toContain("NotReady");
    }
  });

  test("basic pod scheduling works", () => {
    const podName = "smoke-test-pod";
    try {
      kubectl(
        `run ${podName} --image=busybox --restart=Never --command -- echo smoke-ok`
      );
      kubectl(`wait --for=condition=Ready pod/${podName} --timeout=60s 2>/dev/null || true`);
      // Wait for completion
      let status = "";
      for (let i = 0; i < 10; i++) {
        status = kubectl(
          `get pod ${podName} -o jsonpath='{.status.phase}' 2>/dev/null || true`
        );
        if (status === "Succeeded") break;
        execSync("sleep 2");
      }
      const logs = kubectl(`logs ${podName} 2>/dev/null || true`);
      expect(logs).toContain("smoke-ok");
    } finally {
      try {
        kubectl(`delete pod ${podName} --force --grace-period=0 2>/dev/null || true`);
      } catch {
        // cleanup best-effort
      }
    }
  });

  test("Cilium agents are healthy", () => {
    const ciliumPods = kubectl(
      "get pods -n kube-system -l app.kubernetes.io/name=cilium-agent --no-headers"
    );
    const lines = ciliumPods.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBe(6);
    for (const line of lines) {
      expect(line).toContain("Running");
    }
  });

  test("ESO controller is running", () => {
    const esoPods = kubectl(
      "get pods -n external-secrets --no-headers"
    );
    expect(esoPods).toContain("Running");
  });

  // --- Logging pipeline smoke tests ---

  test("Fluent Bit DaemonSet is running", () => {
    const fbPods = kubectl(
      "get pods -n logging -l app=fluent-bit --no-headers"
    );
    const lines = fbPods.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBe(6);
    for (const line of lines) {
      expect(line).toContain("Running");
    }
  });

  test("VictoriaLogs (logs-b) is healthy", () => {
    const result = execSync(
      `curl -sf --connect-timeout 10 -k https://${VICTORIALOGS_HOST}:443/health 2>/dev/null || echo "UNREACHABLE"`,
      { encoding: "utf-8", timeout: 15000 }
    ).trim();
    expect(result).not.toBe("UNREACHABLE");
  });

  test("VictoriaLogs (logs-b) accepts log ingestion", () => {
    const code = execSync(
      `curl -s -o /dev/null -w "%{http_code}" --connect-timeout 10 -k ` +
        `-X POST "https://${VICTORIALOGS_HOST}:443/insert/jsonline?_stream_fields=test&_msg_field=msg" ` +
        `-d '{"msg":"smoke-test-${Date.now()}","test":"smoke","cluster":"${CLUSTER_NAME}"}'`,
      { encoding: "utf-8", timeout: 15000 }
    ).trim();
    expect(code).toBe("200");
  });

  test("VictoriaLogs (logs-b) receives svr00 system logs", () => {
    // Query for svr00 hostname — Fluent Bit on svr00 tags all logs with hostname=svr00
    const result = execSync(
      `curl -sf --connect-timeout 10 -k ` +
        `"https://${VICTORIALOGS_HOST}:443/select/logsql/query?query=hostname%3Asvr00&limit=3" 2>/dev/null || echo ""`,
      { encoding: "utf-8", timeout: 30000 }
    ).trim();
    // On a running system, svr00 system logs should be present
    expect(result.length).toBeGreaterThan(0);
  });
});
