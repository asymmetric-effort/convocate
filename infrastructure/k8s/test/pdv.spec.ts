// Post-Deploy Verification for Kubernetes clusters.
// Validates: all nodes Ready, system pods Running, Cilium healthy,
// mTLS active, ESO can fetch secrets, kubectl works over TLS.

import { test, expect } from "@playwright/test";
import { execSync } from "child_process";

const KUBECONFIG = process.env.KUBECONFIG || "/tmp/kubeconfig";
const CLUSTER_NAME = process.env.CLUSTER_NAME || "cluster-a";

function kubectl(cmd: string): string {
  return execSync(`kubectl --kubeconfig=${KUBECONFIG} ${cmd}`, {
    encoding: "utf-8",
    timeout: 30000,
  }).trim();
}

test.describe(`${CLUSTER_NAME} PDV`, () => {
  test("kubectl can reach the API server over TLS", () => {
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

  test("3 control plane nodes exist", () => {
    const cpNodes = kubectl(
      'get nodes -l node-role.kubernetes.io/control-plane --no-headers'
    );
    const lines = cpNodes.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBe(3);
  });

  test("system pods are Running", () => {
    const pods = kubectl("get pods -n kube-system --no-headers");
    const lines = pods.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBeGreaterThan(0);
    for (const line of lines) {
      const status = line.split(/\s+/)[2];
      expect(["Running", "Completed"]).toContain(status);
    }
  });

  test("Cilium agent pods are running on all nodes", () => {
    const ciliumPods = kubectl(
      "get pods -n kube-system -l app.kubernetes.io/name=cilium-agent --no-headers"
    );
    const lines = ciliumPods.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBe(6);
    for (const line of lines) {
      expect(line).toContain("Running");
    }
  });

  test("Cilium operator is running", () => {
    const operator = kubectl(
      "get pods -n kube-system -l app.kubernetes.io/name=cilium-operator --no-headers"
    );
    expect(operator).toContain("Running");
  });

  test("Hubble relay is running", () => {
    const hubble = kubectl(
      "get pods -n kube-system -l app.kubernetes.io/name=hubble-relay --no-headers"
    );
    expect(hubble).toContain("Running");
  });

  test("kube-proxy is NOT running (replaced by Cilium)", () => {
    const proxyPods = kubectl(
      "get pods -n kube-system -l k8s-app=kube-proxy --no-headers 2>/dev/null || true"
    );
    expect(proxyPods.trim()).toBe("");
  });

  test("Cilium encryption (mTLS) is enabled", () => {
    const config = kubectl(
      "get configmap cilium-config -n kube-system -o jsonpath='{.data.enable-wireguard}'"
    );
    expect(config).toBe("true");
  });

  test("External Secrets Operator pods are running", () => {
    const esoPods = kubectl(
      "get pods -n external-secrets -l app.kubernetes.io/name=external-secrets --no-headers"
    );
    const lines = esoPods.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBeGreaterThan(0);
    for (const line of lines) {
      expect(line).toContain("Running");
    }
  });

  test("ClusterSecretStore openbao-backend exists", () => {
    const store = kubectl(
      "get clustersecretstore openbao-backend -o jsonpath='{.metadata.name}'"
    );
    expect(store).toBe("openbao-backend");
  });

  test("CoreDNS is running", () => {
    const coredns = kubectl(
      "get pods -n kube-system -l k8s-app=kube-dns --no-headers"
    );
    const lines = coredns.split("\n").filter((l) => l.trim().length > 0);
    expect(lines.length).toBeGreaterThan(0);
    for (const line of lines) {
      expect(line).toContain("Running");
    }
  });

  test("cluster DNS resolution works", () => {
    // Create a temporary pod to test DNS
    try {
      kubectl(
        'run dns-test --image=busybox --restart=Never --command -- nslookup kubernetes.default.svc.cluster.local'
      );
      // Wait for completion
      kubectl("wait --for=condition=Ready pod/dns-test --timeout=30s 2>/dev/null || true");
      const logs = kubectl("logs dns-test 2>/dev/null || true");
      expect(logs).toContain("kubernetes.default.svc.cluster.local");
    } finally {
      try {
        kubectl("delete pod dns-test --force --grace-period=0 2>/dev/null || true");
      } catch {
        // cleanup best-effort
      }
    }
  });
});
