package collector

import (
	"fmt"

	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type PodCollector struct {
	pods map[string]*corev1.Pod
}

func NewPodCollector() *PodCollector {
	return &PodCollector{
		pods: make(map[string]*corev1.Pod),
	}
}

func (c *PodCollector) AddPod(pod *corev1.Pod) {
	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	c.pods[key] = pod
	c.collectPodMetrics(pod)
	klog.V(4).Infof("Added pod: %s", key)
}

func (c *PodCollector) DeletePod(pod *corev1.Pod) {
	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	delete(c.pods, key)
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

		if containerStatus.LastTerminationState.Terminated != nil {
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

		if containerStatus.State.Running != nil {
			prevCPU := containerStatus.State.Running.StartedAt.Time
			now := containerStatus.State.Running.StartedAt.Time
			if !prevCPU.IsZero() {
				cpuUsage := now.Sub(prevCPU).Seconds()
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
