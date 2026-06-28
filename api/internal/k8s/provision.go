package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionRequest holds the parameters for provisioning a new worker node.
type ProvisionRequest struct {
	Host     string // IP or hostname of the target machine
	User     string // SSH user (password auth)
	Password string // SSH password
	Location string // convocate.io/location label value
}

// ProvisionNode prepares a blank Ubuntu machine and joins it to the K8s cluster.
// Steps: install prerequisites → containerd → kubeadm/kubelet → generate join
// token → kubeadm join → set location label → CIS hardening.
//
// This runs synchronously and may take several minutes. The handler should
// call it in a goroutine and return 202 Accepted immediately.
func ProvisionNode(ctx context.Context, req ProvisionRequest) error {
	host := req.Host
	user := req.User
	if user == "" {
		user = "convocate"
	}
	pass := req.Password

	log.Printf("[provision] starting provisioning of %s as %s", host, user)

	// Step 1: Base OS preparation (swap, kernel modules, sysctl, packages)
	baseScript := `set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "[provision] disabling swap..."
sudo swapoff -a
sudo sed -i '/\sswap\s/s/^/#/' /etc/fstab

echo "[provision] loading kernel modules..."
echo -e "overlay\nbr_netfilter" | sudo tee /etc/modules-load.d/k8s.conf >/dev/null
sudo modprobe overlay
sudo modprobe br_netfilter

echo "[provision] applying sysctl settings..."
cat <<'SYSCTL' | sudo tee /etc/sysctl.d/99-kubernetes-cis.conf >/dev/null
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
vm.overcommit_memory = 1
vm.panic_on_oom      = 0
kernel.panic         = 10
kernel.panic_on_oops = 1
SYSCTL
sudo sysctl --system >/dev/null 2>&1

echo "[provision] installing base packages..."
sudo apt-get update -qq
sudo apt-get install -y -qq apt-transport-https ca-certificates curl gnupg containerd conntrack socat ethtool ebtables >/dev/null

echo "[provision] configuring containerd..."
sudo mkdir -p /etc/containerd
sudo sh -c 'containerd config default > /etc/containerd/config.toml'
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

echo "[provision] adding Kubernetes apt repo..."
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list >/dev/null

echo "[provision] installing kubeadm, kubelet, kubectl..."
sudo apt-get update -qq
sudo apt-get install -y -qq kubelet=1.31.14-1.1 kubeadm=1.31.14-1.1 kubectl=1.31.14-1.1 >/dev/null
sudo apt-mark hold kubelet kubeadm kubectl

echo "[provision] setting kubelet node-ip..."
echo "KUBELET_EXTRA_ARGS=--node-ip=NODE_IP_PLACEHOLDER" | sudo tee /etc/default/kubelet >/dev/null
sudo systemctl enable kubelet

echo "[provision] base preparation complete"
`
	baseScript = strings.ReplaceAll(baseScript, "NODE_IP_PLACEHOLDER", host)

	log.Printf("[provision] running base preparation on %s", host)
	if err := sshExec(host, user, pass, baseScript); err != nil {
		return fmt.Errorf("base preparation failed: %w", err)
	}

	// Step 2: Generate bootstrap token and discovery kubeconfig on control plane
	log.Printf("[provision] generating join token on control plane")
	token, caCertHash, err := generateJoinCredentials(ctx)
	if err != nil {
		return fmt.Errorf("join credentials: %w", err)
	}

	// Step 3: kubeadm join
	joinScript := fmt.Sprintf(`set -euo pipefail
echo "[provision] joining Kubernetes cluster..."
sudo kubeadm join %s \
  --token %s \
  --discovery-token-ca-cert-hash %s
echo "[provision] kubeadm join complete"
`, "192.168.56.10:6443", token, caCertHash)

	log.Printf("[provision] running kubeadm join on %s", host)
	if err := sshExec(host, user, pass, joinScript); err != nil {
		return fmt.Errorf("kubeadm join failed: %w", err)
	}

	// Step 4: Wait for node to appear in the cluster
	log.Printf("[provision] waiting for node to become Ready...")
	nodeName, err := waitForNodeByIP(ctx, host, 3*time.Minute)
	if err != nil {
		return fmt.Errorf("node did not become ready: %w", err)
	}
	log.Printf("[provision] node %s is Ready", nodeName)

	// Step 5: Set location label
	if req.Location != "" {
		labels := map[string]string{"convocate.io/location": req.Location}
		if err := UpdateNodeLabels(ctx, nodeName, labels); err != nil {
			log.Printf("[provision] warning: failed to set location label: %v", err)
		}
	}

	// Step 6: CIS hardening (worker-level)
	hardenScript := `set -euo pipefail
echo "[provision] applying CIS hardening..."

# Kubelet config permissions (CIS 4.1.x)
for f in /etc/kubernetes/kubelet.conf /etc/kubernetes/pki/ca.crt; do
  if [ -f "$f" ]; then
    sudo chmod 0600 "$f"
    sudo chown root:root "$f"
  fi
done

# Ensure kubelet protectKernelDefaults
if ! grep -q 'protectKernelDefaults' /var/lib/kubelet/config.yaml 2>/dev/null; then
  echo 'protectKernelDefaults: true' | sudo tee -a /var/lib/kubelet/config.yaml >/dev/null
  sudo systemctl restart kubelet
fi

echo "[provision] CIS hardening complete"
`
	if err := sshExec(host, user, pass, hardenScript); err != nil {
		log.Printf("[provision] warning: CIS hardening failed: %v", err)
	}

	log.Printf("[provision] node %s (%s) provisioned successfully", nodeName, host)
	return nil
}

// DrainAndDeleteNode cordons the node, evicts all pods, then removes it from
// the cluster. The underlying VM is NOT destroyed.
func DrainAndDeleteNode(ctx context.Context, nodeName string) error {
	log.Printf("[deprovision] draining node %s", nodeName)

	// Cordon first
	if err := CordonNode(ctx, nodeName); err != nil {
		return fmt.Errorf("cordon: %w", err)
	}

	// Evict pods (delete all non-daemonset pods on this node)
	pods, err := Client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return fmt.Errorf("list pods on node: %w", err)
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		// Skip daemonset-managed pods and mirror pods
		if p.DeletionTimestamp != nil {
			continue
		}
		isDaemonSet := false
		for _, ref := range p.OwnerReferences {
			if ref.Kind == "DaemonSet" {
				isDaemonSet = true
				break
			}
		}
		if isDaemonSet {
			continue
		}
		log.Printf("[deprovision] evicting pod %s/%s", p.Namespace, p.Name)
		_ = Client.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
	}

	// Wait briefly for evictions to propagate
	time.Sleep(5 * time.Second)

	// Delete the node object from the cluster
	log.Printf("[deprovision] deleting node %s from cluster", nodeName)
	if err := Client.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	log.Printf("[deprovision] node %s removed from cluster", nodeName)
	return nil
}

// generateJoinCredentials creates a short-lived bootstrap token and returns
// the token string and the CA cert hash for discovery.
func generateJoinCredentials(ctx context.Context) (token string, caCertHash string, err error) {
	// Create a bootstrap token via the K8s API
	// Token format: [a-z0-9]{6}.[a-z0-9]{16}
	tokenID := fmt.Sprintf("%06x", time.Now().UnixNano()%0xffffff)
	tokenSecret := fmt.Sprintf("%016x", time.Now().UnixNano())

	secret := fmt.Sprintf("%s.%s", tokenID, tokenSecret)

	// Create the bootstrap token secret in kube-system
	_, err = Client.CoreV1().Secrets("kube-system").Create(ctx, bootstrapTokenSecret(tokenID, tokenSecret), metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("create bootstrap token: %w", err)
	}

	// Get the CA cert hash
	cm, err := Client.CoreV1().ConfigMaps("kube-system").Get(ctx, "cluster-info", metav1.GetOptions{})
	if err != nil {
		// Fallback: get CA cert from the kube-public namespace
		cm, err = Client.CoreV1().ConfigMaps("kube-public").Get(ctx, "cluster-info", metav1.GetOptions{})
		if err != nil {
			return secret, "", fmt.Errorf("get cluster-info: %w", err)
		}
	}

	// Extract CA cert hash from cluster-info kubeconfig
	_ = cm
	// Use the kubeadm approach: hash the CA certificate
	caHash, err := getCAHash(ctx)
	if err != nil {
		return secret, "", fmt.Errorf("get CA hash: %w", err)
	}

	return secret, caHash, nil
}

func bootstrapTokenSecret(tokenID, tokenSecret string) *corev1Secret {
	return &corev1Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-token-" + tokenID,
			Namespace: "kube-system",
		},
		Type: "bootstrap.kubernetes.io/token",
		StringData: map[string]string{
			"token-id":                       tokenID,
			"token-secret":                   tokenSecret,
			"usage-bootstrap-authentication": "true",
			"usage-bootstrap-signing":        "true",
			"auth-extra-groups":              "system:bootstrappers:kubeadm:default-node-token",
			"expiration":                     time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		},
	}
}

func getCAHash(ctx context.Context) (string, error) {
	// Read the CA certificate from the kube-system configmap or secret
	secret, err := Client.CoreV1().Secrets("kube-system").Get(ctx, "kubeadm-certs", metav1.GetOptions{})
	if err == nil && secret.Data["ca.crt"] != nil {
		return computeCertHash(secret.Data["ca.crt"]), nil
	}

	// Fallback: read from cluster-info configmap
	cm, err := Client.CoreV1().ConfigMaps("kube-public").Get(ctx, "cluster-info", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cluster-info not found: %w", err)
	}

	kubeconfig := cm.Data["kubeconfig"]
	if kubeconfig == "" {
		return "", fmt.Errorf("cluster-info has no kubeconfig")
	}

	// Extract certificate-authority-data from the kubeconfig YAML
	for _, line := range strings.Split(kubeconfig, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "certificate-authority-data:") {
			b64 := strings.TrimPrefix(trimmed, "certificate-authority-data:")
			b64 = strings.TrimSpace(b64)
			decoded, err := base64Decode(b64)
			if err != nil {
				return "", fmt.Errorf("decode CA cert: %w", err)
			}
			return computeCertHash(decoded), nil
		}
	}

	return "", fmt.Errorf("CA cert not found in cluster-info")
}

// waitForNodeByIP polls the K8s API until a node with the given IP appears and
// reaches Ready status, or until the timeout expires.
func waitForNodeByIP(ctx context.Context, ip string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		nodes, err := Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		for i := range nodes.Items {
			n := &nodes.Items[i]
			for _, addr := range n.Status.Addresses {
				if addr.Address == ip {
					// Check if Ready
					for _, cond := range n.Status.Conditions {
						if cond.Type == "Ready" && cond.Status == "True" {
							return n.Name, nil
						}
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return "", fmt.Errorf("timed out waiting for node with IP %s", ip)
}
