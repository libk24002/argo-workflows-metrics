package collector

import (
	"fmt"
	"sync"

	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type PodCollector struct {
	mu              sync.Mutex
	pods            map[string]*corev1.Pod
	restartCounters map[string]int32
}

func NewPodCollector() *PodCollector {
	return &PodCollector{
		pods:            make(map[string]*corev1.Pod),
		restartCounters: make(map[string]int32),
	}
}

func (c *PodCollector) AddPod(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	podCopy := pod.DeepCopy()

	c.mu.Lock()
	c.pods[key] = podCopy
	c.mu.Unlock()

	c.collectPodMetrics(podCopy)
	klog.V(4).Infof("Added pod: %s", key)
}

func (c *PodCollector) DeletePod(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	c.mu.Lock()
	delete(c.pods, key)
	for _, containerStatus := range pod.Status.ContainerStatuses {
		delete(c.restartCounters, c.containerStateKey(pod, containerStatus.Name))
	}
	c.mu.Unlock()

	c.deletePodMetrics(pod)
	klog.V(4).Infof("Deleted pod: %s", key)
}

func (c *PodCollector) collectPodMetrics(pod *corev1.Pod) {
	namespace := pod.Namespace
	workflowName := pod.Labels["workflows.argoproj.io/workflow"]
	nodeName := pod.Labels["workflows.argoproj.io/workflow-node"]

	if workflowName == "" {
		return
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerName := containerStatus.Name
		stateKey := c.containerStateKey(pod, containerName)

		if containerStatus.LastTerminationState.Terminated != nil {
			c.mu.Lock()
			previousRestartCount := c.restartCounters[stateKey]
			if containerStatus.RestartCount > previousRestartCount {
				c.restartCounters[stateKey] = containerStatus.RestartCount
			}
			c.mu.Unlock()

			if containerStatus.RestartCount <= previousRestartCount {
				continue
			}

			cpuUsage := containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(
				containerStatus.LastTerminationState.Terminated.StartedAt.Time,
			).Seconds()
			if cpuUsage > 0 {
				metrics.ContainerCPUUsageSeconds.WithLabelValues(
					namespace,
					workflowName,
					nodeName,
					containerName,
				).Add(cpuUsage)
			}
		}
	}
}

func (c *PodCollector) deletePodMetrics(pod *corev1.Pod) {
	namespace := pod.Namespace
	workflowName := pod.Labels["workflows.argoproj.io/workflow"]
	nodeName := pod.Labels["workflows.argoproj.io/workflow-node"]

	if workflowName == "" {
		return
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerName := containerStatus.Name
		c.mu.Lock()
		delete(c.restartCounters, c.containerStateKey(pod, containerName))
		c.mu.Unlock()

		metrics.ContainerCPUUsageSeconds.DeleteLabelValues(
			namespace,
			workflowName,
			nodeName,
			containerName,
		)
		metrics.ContainerMemoryUsageBytes.DeleteLabelValues(
			namespace,
			workflowName,
			nodeName,
			containerName,
		)
	}
}

func (c *PodCollector) containerStateKey(pod *corev1.Pod, containerName string) string {
	return fmt.Sprintf("%s/%s/%s/%s", pod.Namespace, string(pod.UID), pod.Name, containerName)
}
