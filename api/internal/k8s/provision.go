package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
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

	// Step 0: Set up passwordless sudo for the provisioning user.
	// The SSH password is used once with sudo -S to install a sudoers
	// drop-in, so all subsequent sudo calls work non-interactively.
	if pass != "" {
		sudoSetup := fmt.Sprintf(
			`echo '%s' | sudo -S sh -c 'echo "%s ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/%s && chmod 0440 /etc/sudoers.d/%s' && echo "[provision] passwordless sudo configured"`,
			pass, user, user, user,
		)
		log.Printf("[provision] configuring passwordless sudo on %s", host)
		if err := sshExecRetry(host, user, pass, sudoSetup, 6, 10*time.Second); err != nil {
			return fmt.Errorf("sudo setup failed: %w", err)
		}
	}

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
if [ ! -f /etc/apt/keyrings/kubernetes-apt-keyring.gpg ]; then
  curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg 2>/dev/null
fi
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list >/dev/null

echo "[provision] installing kubeadm, kubelet, kubectl..."
sudo apt-get update -qq
sudo apt-get install -y -qq kubelet=1.31.14-1.1 kubeadm=1.31.14-1.1 kubectl=1.31.14-1.1 >/dev/null 2>&1
sudo apt-mark hold kubelet kubeadm kubectl >/dev/null 2>&1

echo "[provision] setting kubelet node-ip..."
echo "KUBELET_EXTRA_ARGS=--node-ip=NODE_IP_PLACEHOLDER" | sudo tee /etc/default/kubelet >/dev/null
sudo systemctl enable kubelet

echo "[provision] base preparation complete"
`
	baseScript = strings.ReplaceAll(baseScript, "NODE_IP_PLACEHOLDER", host)

	log.Printf("[provision] running base preparation on %s", host)
	if err := sshExecRetry(host, user, pass, baseScript, 12, 10*time.Second); err != nil {
		return fmt.Errorf("base preparation failed: %w", err)
	}

	// Step 2: Get bootstrap token and discovery kubeconfig from control plane
	log.Printf("[provision] generating join credentials via K8s Job on control plane")
	joinOutput, err := getJoinCommandViaJob(ctx)
	if err != nil {
		return fmt.Errorf("get join credentials: %w", err)
	}

	// Parse token and kubeconfig from output
	var token string
	var discoveryConf string
	for _, line := range strings.Split(joinOutput, "\n") {
		if strings.HasPrefix(line, "TOKEN=") {
			token = strings.TrimPrefix(line, "TOKEN=")
		}
	}
	// Everything after the TOKEN= line is the kubeconfig
	idx := strings.Index(joinOutput, "apiVersion:")
	if idx >= 0 {
		discoveryConf = joinOutput[idx:]
	}

	if token == "" || discoveryConf == "" {
		return fmt.Errorf("failed to parse join credentials from output: %s", joinOutput[:min(200, len(joinOutput))])
	}
	log.Printf("[provision] token=%s, discovery kubeconfig obtained", token)

	// Step 3: Write discovery kubeconfig to target and join
	writeScript := fmt.Sprintf(`sudo mkdir -p /etc/kubernetes
cat > /tmp/discovery.conf << 'DISCOVERY_EOF'
%s
DISCOVERY_EOF
sudo mv /tmp/discovery.conf /etc/kubernetes/discovery.conf
sudo chmod 0600 /etc/kubernetes/discovery.conf
echo "[provision] discovery config written"
`, discoveryConf)

	if err := sshExec(host, user, pass, writeScript); err != nil {
		return fmt.Errorf("write discovery config: %w", err)
	}

	joinScript := fmt.Sprintf(`set -euo pipefail
echo "[provision] joining Kubernetes cluster..."
sudo kubeadm join \
  --discovery-file /etc/kubernetes/discovery.conf \
  --tls-bootstrap-token %s \
  --v=2 2>&1 || { echo "[provision] kubeadm join FAILED with exit $?"; exit 1; }
sudo rm -f /etc/kubernetes/discovery.conf
echo "[provision] kubeadm join complete"
`, token)

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

	fullToken := fmt.Sprintf("%s.%s", tokenID, tokenSecret)

	// Create the bootstrap token secret in kube-system
	_, err = Client.CoreV1().Secrets("kube-system").Create(ctx, bootstrapTokenSecret(tokenID, tokenSecret), metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("create bootstrap token: %w", err)
	}

	// Get the CA cert hash from cluster-info configmap
	cm, err := Client.CoreV1().ConfigMaps("kube-public").Get(ctx, "cluster-info", metav1.GetOptions{})
	if err != nil {
		return fullToken, "", fmt.Errorf("get cluster-info: %w", err)
	}

	kubeconfig := cm.Data["kubeconfig"]
	if kubeconfig == "" {
		return fullToken, "", fmt.Errorf("cluster-info has no kubeconfig")
	}

	// Extract certificate-authority-data and compute hash
	for _, line := range strings.Split(kubeconfig, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "certificate-authority-data:") {
			b64 := strings.TrimPrefix(trimmed, "certificate-authority-data:")
			b64 = strings.TrimSpace(b64)
			certPEM, decErr := base64Decode(b64)
			if decErr != nil {
				return fullToken, "", fmt.Errorf("decode CA cert: %w", decErr)
			}
			// kubeadm hashes the DER-encoded public key, but for discovery
			// via --discovery-token-ca-cert-hash it wants sha256 of the
			// entire DER-encoded SubjectPublicKeyInfo. We can use the PEM
			// cert's raw DER for a simpler approach that kubeadm also accepts.
			caHash := computeCertHash(certPEM)
			return fullToken, caHash, nil
		}
	}

	return fullToken, "", fmt.Errorf("CA cert not found in cluster-info")
}

// getJoinCommandViaJob creates a privileged Pod on a control-plane node to
// run `kubeadm token create --print-join-command`, captures the output, and
// cleans up. This is the CIS-compliant way to get a join command since
// anonymous auth is disabled.
func getJoinCommandViaJob(ctx context.Context) (string, error) {
	podName := fmt.Sprintf("convocate-join-token-%d", time.Now().Unix())
	privileged := true
	hostPathDir := corev1.HostPathDirectory

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			HostNetwork:   true,
			HostPID:       true,
			NodeSelector:  map[string]string{"node-role.kubernetes.io/control-plane": ""},
			Tolerations: []corev1.Toleration{
				{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			},
			Containers: []corev1.Container{{
				Name:  "kubeadm",
				Image: "ubuntu:24.04",
				Command: []string{"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--", "sh", "-c",
					// Generate a signed discovery kubeconfig with the bootstrap token
					// as an embedded credential so kubeadm join can authenticate.
					"TOKEN=$(kubeadm token create 2>/dev/null) && " +
						"CA_B64=$(base64 -w0 /etc/kubernetes/pki/ca.crt) && " +
						"echo \"TOKEN=$TOKEN\" && " +
						"cat <<EOF\n" +
						"apiVersion: v1\n" +
						"clusters:\n" +
						"- cluster:\n" +
						"    certificate-authority-data: $CA_B64\n" +
						"    server: https://192.168.56.10:6443\n" +
						"  name: cluster\n" +
						"contexts:\n" +
						"- context:\n" +
						"    cluster: cluster\n" +
						"    user: bootstrap\n" +
						"  name: bootstrap\n" +
						"current-context: bootstrap\n" +
						"kind: Config\n" +
						"preferences: {}\n" +
						"users:\n" +
						"- name: bootstrap\n" +
						"  user:\n" +
						"    token: $TOKEN\n" +
						"EOF"},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "etc-kubernetes", MountPath: "/etc/kubernetes", ReadOnly: true},
				},
			}},
			Volumes: []corev1.Volume{
				{Name: "etc-kubernetes", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/kubernetes", Type: &hostPathDir}}},
			},
		},
	}

	_, err := Client.CoreV1().Pods("kube-system").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create join-token pod: %w", err)
	}
	defer func() {
		_ = Client.CoreV1().Pods("kube-system").Delete(ctx, podName, metav1.DeleteOptions{})
	}()

	// Wait for the pod to complete
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		p, err := Client.CoreV1().Pods("kube-system").Get(ctx, podName, metav1.GetOptions{})
		if err == nil {
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}

	// Read logs
	logReq := Client.CoreV1().Pods("kube-system").GetLogs(podName, &corev1.PodLogOptions{})
	logStream, err := logReq.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("get join-token logs: %w", err)
	}
	defer logStream.Close()

	var buf strings.Builder
	bufBytes := make([]byte, 4096)
	for {
		n, readErr := logStream.Read(bufBytes)
		if n > 0 {
			buf.Write(bufBytes[:n])
		}
		if readErr != nil {
			break
		}
	}

	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "TOKEN=") {
		return "", fmt.Errorf("unexpected join output: %s", output)
	}

	return output, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
