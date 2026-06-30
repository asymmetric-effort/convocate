package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/asymmetric-effort/convocate/internal/types"
)

const defaultAgentImage = "192.168.3.90:5000/convocate/agent:latest"

// Default resource values for agent pods
const (
	defaultCPURequest    = "500m"
	defaultCPULimit      = "2"
	defaultMemRequest    = "512Mi"
	defaultMemLimit      = "2Gi"
	defaultStorageSize   = "2Gi"
	defaultClaudeMd      = "# Convocate Agent\n\nYou are a Convocate agent. Follow all instructions carefully.\n"
)

func EnsureAgentNamespace(ctx context.Context) error {
	_, err := Client.CoreV1().Namespaces().Get(ctx, AgentNamespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace},
	}
	_, err = Client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func ListAgentPods(ctx context.Context) ([]types.Agent, error) {
	pods, err := Client.CoreV1().Pods(AgentNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "convocate.io/type=agent",
	})
	if err != nil {
		return nil, fmt.Errorf("list agent pods: %w", err)
	}

	var agents []types.Agent
	for i := range pods.Items {
		agents = append(agents, podToAgent(&pods.Items[i]))
	}
	return agents, nil
}

func GetAgentPod(ctx context.Context, name string) (*types.Agent, error) {
	pod, err := Client.CoreV1().Pods(AgentNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get agent pod %s: %w", name, err)
	}
	agent := podToAgent(pod)
	return &agent, nil
}

// CreateAgentPVC creates a dedicated PersistentVolumeClaim for an agent pod.
func CreateAgentPVC(ctx context.Context, podName, storageSize string) error {
	if storageSize == "" {
		storageSize = defaultStorageSize
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-" + podName,
			Namespace: AgentNamespace,
			Labels: map[string]string{
				"convocate.io/type":  "agent-pvc",
				"convocate.io/agent": podName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}
	_, err := Client.CoreV1().PersistentVolumeClaims(AgentNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create agent PVC: %w", err)
	}
	return nil
}

// CreateAgentConfigMap creates a ConfigMap with the CLAUDE.md content.
func CreateAgentConfigMap(ctx context.Context, podName, claudeMd string) error {
	if claudeMd == "" {
		claudeMd = defaultClaudeMd
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-" + podName,
			Namespace: AgentNamespace,
			Labels: map[string]string{
				"convocate.io/type":  "agent-config",
				"convocate.io/agent": podName,
			},
		},
		Data: map[string]string{
			"CLAUDE.md": claudeMd,
		},
	}
	_, err := Client.CoreV1().ConfigMaps(AgentNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create agent ConfigMap: %w", err)
	}
	return nil
}

// UpdateAgentConfigMap updates the CLAUDE.md content in an existing ConfigMap.
// This triggers the fsnotify watcher in the agent wrapper to restart Claude CLI.
func UpdateAgentConfigMap(ctx context.Context, podName, claudeMd string) error {
	cmName := "cm-" + podName
	cm, err := Client.CoreV1().ConfigMaps(AgentNamespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get agent ConfigMap: %w", err)
	}
	cm.Data["CLAUDE.md"] = claudeMd
	_, err = Client.CoreV1().ConfigMaps(AgentNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// CreateAgentSecret creates a K8s Secret for the Anthropic API key.
func CreateAgentSecret(ctx context.Context, podName, apiKey string) error {
	if apiKey == "" {
		return nil // No secret needed
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-" + podName,
			Namespace: AgentNamespace,
			Labels: map[string]string{
				"convocate.io/type":  "agent-secret",
				"convocate.io/agent": podName,
			},
		},
		StringData: map[string]string{
			"ANTHROPIC_API_KEY": apiKey,
		},
	}
	_, err := Client.CoreV1().Secrets(AgentNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create agent Secret: %w", err)
	}
	return nil
}

// CreateAgentPod creates a fully-specified agent pod with PVC, ConfigMap,
// Secret, security context, resource limits, and probes.
func CreateAgentPod(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
	image := req.Image
	if image == "" {
		image = defaultAgentImage
	}

	podName := fmt.Sprintf("agent-%s", req.Project)
	labels := map[string]string{
		"convocate.io/type":    "agent",
		"convocate.io/project": req.Project,
		"convocate.io/owner":   owner,
		"app":                  "convocate-agent",
	}

	// Create supporting resources
	storageSize := defaultStorageSize
	if req.Resources != nil && req.Resources.StorageSize != "" {
		storageSize = req.Resources.StorageSize
	}
	if err := CreateAgentPVC(ctx, podName, storageSize); err != nil {
		return nil, err
	}
	if err := CreateAgentConfigMap(ctx, podName, req.ClaudeMd); err != nil {
		return nil, err
	}
	if err := CreateAgentSecret(ctx, podName, req.AnthropicApiKey); err != nil {
		return nil, err
	}
	if err := CreateAgentCertificate(ctx, podName); err != nil {
		log.Printf("[agent] Warning: TLS certificate creation failed: %v", err)
		// Non-fatal — wrapper falls back to HTTP dev mode
	}

	// Build Claude flags env var
	claudeFlags := strings.Join(req.ClaudeFlags, " ")

	// Build environment variables
	env := []corev1.EnvVar{
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
		}},
		{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
		}},
		{Name: "CLAUDE_FLAGS", Value: claudeFlags},
		{Name: "LISTEN_ADDR", Value: ":8443"},
		{Name: "WORK_DIR", Value: "/home/claude/workspace"},
		{Name: "CLAUDE_MD_PATH", Value: "/home/claude/CLAUDE.md"},
	}

	// Add API key from secret if provided
	if req.AnthropicApiKey != "" {
		env = append(env, corev1.EnvVar{
			Name: "ANTHROPIC_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-" + podName},
					Key:                  "ANTHROPIC_API_KEY",
				},
			},
		})
	}

	// Add logging flag
	if req.Logging {
		env = append(env, corev1.EnvVar{Name: "LOGGING_ENABLED", Value: "true"})
	}

	// Build resource requirements
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(valueOr(req.Resources, func(r *types.AgentResources) string { return r.CPURequest }, defaultCPURequest)),
			corev1.ResourceMemory: resource.MustParse(valueOr(req.Resources, func(r *types.AgentResources) string { return r.MemoryRequest }, defaultMemRequest)),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(valueOr(req.Resources, func(r *types.AgentResources) string { return r.CPULimit }, defaultCPULimit)),
			corev1.ResourceMemory: resource.MustParse(valueOr(req.Resources, func(r *types.AgentResources) string { return r.MemoryLimit }, defaultMemLimit)),
		},
	}

	// Volume mounts
	volumeMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/home/claude/workspace"},
		{Name: "claude-md", MountPath: "/home/claude/CLAUDE.md", SubPath: "CLAUDE.md", ReadOnly: true},
		{Name: "tls", MountPath: "/etc/tls", ReadOnly: true},
		{Name: "tmp", MountPath: "/tmp"},
	}

	// Volumes
	volumes := []corev1.Volume{
		{Name: "workspace", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-" + podName},
		}},
		{Name: "claude-md", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "cm-" + podName},
			},
		}},
		{Name: "tls", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "tls-" + podName,
				Optional:   boolPtr(true), // Allow pod to start without TLS (dev mode)
			},
		}},
		{Name: "tmp", VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory},
		}},
	}

	// Security context
	allowPrivEsc := false
	readOnlyFS := true
	runAsNonRoot := true
	var uid int64 = 1337
	var gid int64 = 1337
	var gracePeriod int64 = 30

	containerSC := &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivEsc,
		ReadOnlyRootFilesystem:   &readOnlyFS,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	// Apply admin-only security overrides
	if req.Security != nil {
		if len(req.Security.Capabilities) > 0 {
			var caps []corev1.Capability
			for _, c := range req.Security.Capabilities {
				caps = append(caps, corev1.Capability(c))
			}
			containerSC.Capabilities.Add = caps
		}
		if req.Security.DockerAccess {
			// Mount Docker socket
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: "docker-sock", MountPath: "/var/run/docker.sock",
			})
			volumes = append(volumes, corev1.Volume{
				Name: "docker-sock", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/var/run/docker.sock"},
				},
			})
		}
		for _, m := range req.Security.AdditionalMounts {
			mountName := fmt.Sprintf("extra-%d", len(volumeMounts))
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: mountName, MountPath: m.MountPath, ReadOnly: m.ReadOnly,
			})
			hostPathType := corev1.HostPathDirectory
			volumes = append(volumes, corev1.Volume{
				Name: mountName, VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: m.HostPath, Type: &hostPathType},
				},
			})
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: AgentNamespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName:            "convocate-agent",
			TerminationGracePeriodSeconds: &gracePeriod,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &uid,
				RunAsGroup:   &gid,
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{{
				Name:            "agent",
				Image:           image,
				ImagePullPolicy: corev1.PullAlways,
				Command:         []string{"/usr/bin/convocate-agent-wrapper"},
				Env:             env,
				Ports: []corev1.ContainerPort{{
					ContainerPort: 8443,
					Name:          "https",
					Protocol:      corev1.ProtocolTCP,
				}},
				Resources:       resources,
				VolumeMounts:    volumeMounts,
				SecurityContext: containerSC,
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/healthz",
							Port:   intstr8443(),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 10,
					PeriodSeconds:       10,
					FailureThreshold:    3,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/readyz",
							Port:   intstr8443(),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       5,
					FailureThreshold:    3,
				},
			}},
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if req.NodeID != "" {
		pod.Spec.NodeSelector = map[string]string{
			"kubernetes.io/hostname": req.NodeID,
		}
	}

	log.Printf("[agent] Creating agent pod %s (image=%s, flags=%v)", podName, image, req.ClaudeFlags)
	created, err := Client.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create agent pod: %w", err)
	}

	agent := podToAgent(created)
	return &agent, nil
}

// DeleteAgentPod deletes the agent pod and all associated resources
// (PVC, ConfigMap, Secret).
func DeleteAgentPod(ctx context.Context, name string) error {
	// Delete pod
	err := Client.CoreV1().Pods(AgentNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("[agent] Warning: delete pod %s: %v", name, err)
	}

	// Clean up associated resources
	cleanupResource := func(kind, resName string, deleteFn func() error) {
		if delErr := deleteFn(); delErr != nil {
			log.Printf("[agent] Warning: delete %s %s: %v", kind, resName, delErr)
		}
	}

	cleanupResource("PVC", "pvc-"+name, func() error {
		return Client.CoreV1().PersistentVolumeClaims(AgentNamespace).Delete(ctx, "pvc-"+name, metav1.DeleteOptions{})
	})
	cleanupResource("ConfigMap", "cm-"+name, func() error {
		return Client.CoreV1().ConfigMaps(AgentNamespace).Delete(ctx, "cm-"+name, metav1.DeleteOptions{})
	})
	cleanupResource("Secret", "secret-"+name, func() error {
		return Client.CoreV1().Secrets(AgentNamespace).Delete(ctx, "secret-"+name, metav1.DeleteOptions{})
	})
	// Clean up TLS certificate and its secret
	if DynClient != nil {
		certGVR := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
		cleanupResource("Certificate", "tls-"+name, func() error {
			return DynClient.Resource(certGVR).Namespace(AgentNamespace).Delete(ctx, "tls-"+name, metav1.DeleteOptions{})
		})
	}
	cleanupResource("TLS Secret", "tls-"+name, func() error {
		return Client.CoreV1().Secrets(AgentNamespace).Delete(ctx, "tls-"+name, metav1.DeleteOptions{})
	})

	return err
}

// CreateAgentCertificate creates a cert-manager Certificate for per-pod TLS.
// The certificate is issued by the convocate-agent-pod-ca issuer.
func CreateAgentCertificate(ctx context.Context, podName string) error {
	if DynClient == nil {
		log.Printf("[agent] Dynamic client not available — skipping TLS certificate creation")
		return nil
	}

	certGVR := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}

	cert := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "tls-" + podName,
				"namespace": AgentNamespace,
				"labels": map[string]interface{}{
					"convocate.io/type":  "agent-tls",
					"convocate.io/agent": podName,
				},
			},
			"spec": map[string]interface{}{
				"secretName": "tls-" + podName,
				"issuerRef": map[string]interface{}{
					"name": "convocate-agent-pod-ca",
					"kind": "Issuer",
				},
				"dnsNames": []interface{}{
					podName,
					podName + "." + AgentNamespace + ".svc.cluster.local",
				},
				"duration":    "8760h",
				"renewBefore": "720h",
			},
		},
	}

	_, err := DynClient.Resource(certGVR).Namespace(AgentNamespace).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create agent certificate: %w", err)
	}
	return nil
}

// StopAgentPod deletes only the pod (not PVC/ConfigMap/Secret).
// Used when stopping an agent — the persistent state is preserved.
func StopAgentPod(ctx context.Context, name string) error {
	return Client.CoreV1().Pods(AgentNamespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// GetOpts returns default GetOptions.
func GetOpts() metav1.GetOptions {
	return metav1.GetOptions{}
}

// valueOr extracts a field from resources or returns a default.
func valueOr(r *types.AgentResources, getter func(*types.AgentResources) string, defaultVal string) string {
	if r != nil {
		if v := getter(r); v != "" {
			return v
		}
	}
	return defaultVal
}

func boolPtr(b bool) *bool { return &b }

// intstr8443 returns an IntOrString for port 8443.
func intstr8443() intstr.IntOrString {
	return intstr.FromInt32(8443)
}
