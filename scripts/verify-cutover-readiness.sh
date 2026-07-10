#!/bin/bash
# Verify all prerequisites for svr00 cutover are met
set -e

echo "=== Cutover Readiness Check ==="

echo "1. Ansible roles present..."
for role in vm-provision common containerd kubeadm-init kubeadm-join-cp kubeadm-join-worker kube-vip cilium cert-manager cis-hardening cloudflared openbao registry openbao-pki; do
  [ -f "infrastructure/roles/$role/tasks/main.yml" ] && echo "  ✅ $role" || echo "  ❌ $role MISSING"
done

echo "2. Helm chart present..."
helm lint infrastructure/charts/convocate/ 2>&1 | tail -1

echo "3. CD pipeline present..."
[ -f ".github/workflows/cd.yml" ] && echo "  ✅ cd.yml" || echo "  ❌ cd.yml MISSING"

echo "4. Nightly recycle present..."
[ -f ".github/workflows/nightly-recycle.yml" ] && echo "  ✅ nightly-recycle.yml" || echo "  ❌ MISSING"

echo "5. Leakdetector configured..."
[ -f ".leakdetector.yml" ] && echo "  ✅ .leakdetector.yml" || echo "  ❌ MISSING"

echo "6. No hardcoded secrets..."
count=$(grep -rn "convocate-influx\|convocate-dev-secret\|convocate-grafana" k8s/ --include="*.yaml" 2>/dev/null | wc -l)
[ "$count" -eq 0 ] && echo "  ✅ Clean" || echo "  ❌ $count secrets found"

echo "7. No mock auth..."
count=$(grep -rn "ALLOW_MOCK_AUTH" k8s/ api/ --include="*.yaml" --include="*.go" 2>/dev/null | grep -v "_test.go" | wc -l)
[ "$count" -eq 0 ] && echo "  ✅ Clean" || echo "  ❌ $count references found"

echo ""
echo "=== Ready for cutover: all checks must be ✅ ==="
