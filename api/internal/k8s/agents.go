package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/asymmetric-effort/convocate/internal/types"
)

const defaultAgentImage = "192.168.3.90:5000/convocate/agent:latest"

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
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: AgentNamespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "agent",
				Image:   image,
				Command: []string{"claude", "--dangerously-skip-permissions"},
				Stdin:   true,
				TTY:     true,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if req.NodeID != "" {
		pod.Spec.NodeSelector = map[string]string{
			"kubernetes.io/hostname": req.NodeID,
		}
	}

	created, err := Client.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create agent pod: %w", err)
	}

	agent := podToAgent(created)
	return &agent, nil
}

func DeleteAgentPod(ctx context.Context, name string) error {
	return Client.CoreV1().Pods(AgentNamespace).Delete(ctx, name, metav1.DeleteOptions{})
}
