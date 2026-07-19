#!/usr/bin/env bash
# One-time setup: configure all K8s nodes to trust the private registry at ghcr.io/asymmetric-effort.
# Run from the build host (192.168.3.90). Requires SSH access to svr00 (192.168.3.159).
set -euo pipefail

REGISTRY_HOST="ghcr.io/asymmetric-effort"
SVR00="samcaldwell@192.168.3.159"
VAGRANT_DIR="/home/samcaldwell/git/svr00"
NODES="convocate01 convocate02 convocate03 convocate04 convocate05 convocate06"

HOSTS_TOML="server = \"http://${REGISTRY_HOST}\"

[host.\"http://${REGISTRY_HOST}\"]
  capabilities = [\"pull\", \"resolve\"]
  skip_verify = true
"

echo "Configuring containerd on all K8s nodes to trust ${REGISTRY_HOST}..."

for node in $NODES; do
    echo "  Configuring ${node}..."
    ssh "$SVR00" "cd ${VAGRANT_DIR} && vagrant ssh ${node} -c '
        sudo mkdir -p /etc/containerd/certs.d/${REGISTRY_HOST}
        echo \"${HOSTS_TOML}\" | sudo tee /etc/containerd/certs.d/${REGISTRY_HOST}/hosts.toml > /dev/null
        sudo systemctl restart containerd
        echo \"  ${node}: containerd restarted\"
    '" 2>&1 | grep -v "^$" || true
done

echo ""
echo "All nodes configured. Verifying pull access..."

# Test pull on one worker node
ssh "$SVR00" "cd ${VAGRANT_DIR} && vagrant ssh convocate04 -c '
    sudo crictl pull ${REGISTRY_HOST}/convocate/registry:latest 2>&1
'" 2>&1 || echo "Warning: pull test failed — registry may not be running yet"

echo "Done."
