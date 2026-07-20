# Kubernetes Cluster Infrastructure

Two Kubernetes clusters running on libvirt VMs on svr00 (192.168.3.159).

## Architecture

### Cluster A (Canary)
- 6 VMs: node-a-0 through node-a-5
- Control plane: node-a-0, node-a-1, node-a-2
- Workers: node-a-3, node-a-4, node-a-5
- Cluster network: 192.168.55.0/24 (libvirt bridge "cluster-a")
- LAN IPs: 192.168.3.170-175
- Pod CIDR: 10.55.0.0/16

### Cluster B (Production)
- 6 VMs: node-b-0 through node-b-5
- Control plane: node-b-0, node-b-1, node-b-2
- Workers: node-b-3, node-b-4, node-b-5
- Cluster network: 192.168.56.0/24 (libvirt bridge "cluster-b")
- LAN IPs: 192.168.3.180-185
- Pod CIDR: 10.56.0.0/16

### VM Specifications
- 8 GB RAM, 8 vCPU per VM
- Disks: 100 GB root (/), 10 GB /var/log, 5 GB /tmp (noexec)
- Ubuntu 24.04 server
- Two NICs: cluster bridge + macvtap on enp4s0f0 (LAN)
- User: convocate (passwordless sudo)
- SSH keys: sam-caldwell GitHub keys + CI key

### Networking
- kubeadm for cluster bootstrap (v1.31)
- Cilium CNI (replaces kube-proxy, provides service mesh with mTLS)
- Cilium uses OpenBao PKI (secrets-b at 192.168.3.161:443) as external CA
- External Secrets Operator for K8s secrets from OpenBao

## Deployment Order

1. `provision-vms.yml` - Create VMs via virt-install on svr00
2. `configure-nodes.yml` - OS config, networking, containerd, kubeadm
3. `bootstrap-cluster.yml` - kubeadm init + join (3 CP + 3 workers)
4. `install-cilium.yml` - Cilium CNI with OpenBao PKI integration
5. `install-eso.yml` - External Secrets Operator with OpenBao backend
6. `deploy.yml` - Full deploy (runs all above in order)

## CI/CD

Changes to `infrastructure/k8s/**` trigger:
1. Deploy cluster-a (canary, wipe OK)
2. PDV tests on cluster-a
3. Deploy cluster-b (production, zero-downtime only)
4. Smoke tests on cluster-b
