package framework

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodLog returns the tail of a container log.
func (h *Harness) PodLog(ctx context.Context, namespace, pod, container string, tail int64) (string, error) {
	options := &corev1.PodLogOptions{Container: container}
	if tail > 0 {
		options.TailLines = &tail
	}

	stream, err := h.Clientset.CoreV1().Pods(namespace).GetLogs(pod, options).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("opening the log of %s/%s container %s: %w", namespace, pod, container, err)
	}

	defer func() { _ = stream.Close() }()

	content, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading the log of %s/%s container %s: %w", namespace, pod, container, err)
	}

	return string(content), nil
}

// PodsForLabels lists the pods a selector matches.
func (h *Harness) PodsForLabels(ctx context.Context, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}

	err := h.Client.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(labels))
	if err != nil {
		return nil, fmt.Errorf("listing the pods of %s matching %v: %w", namespace, labels, err)
	}

	return pods.Items, nil
}
