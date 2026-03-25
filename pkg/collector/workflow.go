package collector

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	"k8s.io/klog/v2"
)

// WorkflowCollector collects metrics from Workflow resources
type WorkflowCollector struct {
	mu                  sync.RWMutex
	workflows           map[string]*wfv1.Workflow
	lastNamespaces      map[string]struct{}
	lastNamespacePhases map[string]map[string]struct{}
}

// NewWorkflowCollector creates a new WorkflowCollector
func NewWorkflowCollector() *WorkflowCollector {
	return &WorkflowCollector{
		workflows:           make(map[string]*wfv1.Workflow),
		lastNamespaces:      make(map[string]struct{}),
		lastNamespacePhases: make(map[string]map[string]struct{}),
	}
}

// AddWorkflow adds or updates a workflow in the collector
func (c *WorkflowCollector) AddWorkflow(wf *wfv1.Workflow) {
	if wf == nil {
		return
	}

	key := fmt.Sprintf("%s/%s", wf.Namespace, wf.Name)
	wfCopy := wf.DeepCopy()

	c.mu.Lock()
	previous := c.workflows[key]
	c.workflows[key] = wfCopy
	c.mu.Unlock()

	if previous != nil {
		c.deleteWorkflowSeries(previous)
	}

	c.collectWorkflowMetrics(wfCopy)
	klog.V(4).Infof("Added workflow: %s", key)
}

// DeleteWorkflow removes a workflow from the collector
func (c *WorkflowCollector) DeleteWorkflow(wf *wfv1.Workflow) {
	if wf == nil {
		return
	}

	key := fmt.Sprintf("%s/%s", wf.Namespace, wf.Name)

	c.mu.Lock()
	cached := c.workflows[key]
	delete(c.workflows, key)
	c.mu.Unlock()

	target := wf
	if cached != nil {
		target = cached
	}

	c.deleteWorkflowSeries(target)
	c.updateAggregatedMetrics()
	klog.V(4).Infof("Deleted workflow: %s", key)
}

// collectWorkflowMetrics collects metrics for a single workflow
func (c *WorkflowCollector) collectWorkflowMetrics(wf *wfv1.Workflow) {
	namespace := wf.Namespace
	name := wf.Name
	phase := string(wf.Status.Phase)

	// Set workflow status phase
	for _, p := range workflowPhases {
		value := 0.0
		if p == phase {
			value = 1.0
		}
		metrics.WorkflowStatusPhase.WithLabelValues(namespace, name, p).Set(value)
	}

	// Set workflow info
	priority := "0"
	if wf.Spec.Priority != nil {
		priority = strconv.Itoa(int(*wf.Spec.Priority))
	}
	serviceAccount := wf.Spec.ServiceAccountName
	if serviceAccount == "" {
		serviceAccount = "default"
	}
	metrics.WorkflowInfo.WithLabelValues(
		namespace,
		name,
		string(wf.UID),
		serviceAccount,
		priority,
	).Set(1)

	// Set workflow created time
	if !wf.CreationTimestamp.IsZero() {
		metrics.WorkflowCreatedTime.WithLabelValues(namespace, name).Set(
			float64(wf.CreationTimestamp.Unix()),
		)
	}

	// Set workflow started time
	if !wf.Status.StartedAt.IsZero() {
		metrics.WorkflowStartedTime.WithLabelValues(namespace, name).Set(
			float64(wf.Status.StartedAt.Unix()),
		)
	}

	// Set workflow finished time
	if !wf.Status.FinishedAt.IsZero() {
		metrics.WorkflowFinishedTime.WithLabelValues(namespace, name).Set(
			float64(wf.Status.FinishedAt.Unix()),
		)
	}

	// Calculate and set workflow duration
	if !wf.Status.StartedAt.IsZero() {
		var duration time.Duration
		if !wf.Status.FinishedAt.IsZero() {
			duration = wf.Status.FinishedAt.Sub(wf.Status.StartedAt.Time)
		} else if phase == "Running" {
			duration = time.Since(wf.Status.StartedAt.Time)
		}
		if duration > 0 {
			metrics.WorkflowDurationSeconds.WithLabelValues(namespace, name, phase).Set(
				duration.Seconds(),
			)
		}
	}

	// Collect node metrics
	c.collectNodeMetrics(wf)

	c.updateAggregatedMetrics()
}

// collectNodeMetrics collects metrics for workflow nodes
func (c *WorkflowCollector) collectNodeMetrics(wf *wfv1.Workflow) {
	namespace := wf.Namespace
	name := wf.Name

	if wf.Status.Nodes == nil {
		return
	}

	// Count total nodes
	totalNodes := len(wf.Status.Nodes)
	metrics.WorkflowNodeTotal.WithLabelValues(namespace, name).Set(float64(totalNodes))

	// Count nodes by phase
	nodePhaseCount := make(map[string]int)
	for _, node := range wf.Status.Nodes {
		phase := string(node.Phase)
		nodePhaseCount[phase]++
	}

	// Set node phase metrics
	for phase, count := range nodePhaseCount {
		metrics.WorkflowNodePhase.WithLabelValues(namespace, name, phase).Set(float64(count))
	}
}

func (c *WorkflowCollector) deleteWorkflowSeries(wf *wfv1.Workflow) {
	namespace := wf.Namespace
	name := wf.Name

	for _, p := range workflowPhases {
		metrics.WorkflowStatusPhase.DeleteLabelValues(namespace, name, p)
	}

	// Delete workflow info
	priority := "0"
	if wf.Spec.Priority != nil {
		priority = strconv.Itoa(int(*wf.Spec.Priority))
	}
	serviceAccount := wf.Spec.ServiceAccountName
	if serviceAccount == "" {
		serviceAccount = "default"
	}
	metrics.WorkflowInfo.DeleteLabelValues(namespace, name, string(wf.UID), serviceAccount, priority)

	// Delete time metrics
	metrics.WorkflowCreatedTime.DeleteLabelValues(namespace, name)
	metrics.WorkflowStartedTime.DeleteLabelValues(namespace, name)
	metrics.WorkflowFinishedTime.DeleteLabelValues(namespace, name)

	// Delete duration metrics
	phase := string(wf.Status.Phase)
	metrics.WorkflowDurationSeconds.DeleteLabelValues(namespace, name, phase)

	// Delete node metrics
	metrics.WorkflowNodeTotal.DeleteLabelValues(namespace, name)
	if wf.Status.Nodes != nil {
		nodePhases := make(map[string]bool)
		for _, node := range wf.Status.Nodes {
			nodePhases[string(node.Phase)] = true
		}
		for phase := range nodePhases {
			metrics.WorkflowNodePhase.DeleteLabelValues(namespace, name, phase)
		}
	}

}

// updateAggregatedMetrics updates cluster-wide aggregated metrics
func (c *WorkflowCollector) updateAggregatedMetrics() {
	namespaceCounts := make(map[string]int)
	namespacePhaseCount := make(map[string]map[string]int)
	currentNamespaces := make(map[string]struct{})
	currentNamespacePhases := make(map[string]map[string]struct{})
	var staleNamespaces map[string]struct{}
	var staleNamespacePhases map[string]map[string]struct{}

	c.mu.Lock()
	for _, wf := range c.workflows {
		namespace := wf.Namespace
		phase := string(wf.Status.Phase)

		namespaceCounts[namespace]++
		currentNamespaces[namespace] = struct{}{}

		if namespacePhaseCount[namespace] == nil {
			namespacePhaseCount[namespace] = make(map[string]int)
		}
		namespacePhaseCount[namespace][phase]++

		if currentNamespacePhases[namespace] == nil {
			currentNamespacePhases[namespace] = make(map[string]struct{})
		}
		currentNamespacePhases[namespace][phase] = struct{}{}
	}

	staleNamespaces = diffNamespaces(c.lastNamespaces, currentNamespaces)
	staleNamespacePhases = diffNamespacePhases(c.lastNamespacePhases, currentNamespacePhases)

	c.lastNamespaces = currentNamespaces
	c.lastNamespacePhases = currentNamespacePhases
	c.mu.Unlock()

	for namespace, count := range namespaceCounts {
		metrics.WorkflowCount.WithLabelValues(namespace).Set(float64(count))
	}
	for namespace := range staleNamespaces {
		metrics.WorkflowCount.DeleteLabelValues(namespace)
	}

	for namespace, phaseCount := range namespacePhaseCount {
		for phase, count := range phaseCount {
			metrics.WorkflowStatusTotal.WithLabelValues(namespace, phase).Set(float64(count))
		}
	}
	for namespace, phases := range staleNamespacePhases {
		for phase := range phases {
			metrics.WorkflowStatusTotal.DeleteLabelValues(namespace, phase)
		}
	}
}

func diffNamespaces(previous, current map[string]struct{}) map[string]struct{} {
	stale := make(map[string]struct{})
	for namespace := range previous {
		if _, ok := current[namespace]; ok {
			continue
		}
		stale[namespace] = struct{}{}
	}
	return stale
}

func diffNamespacePhases(previous, current map[string]map[string]struct{}) map[string]map[string]struct{} {
	stale := make(map[string]map[string]struct{})
	for namespace, phases := range previous {
		for phase := range phases {
			if currentPhases, ok := current[namespace]; ok {
				if _, exists := currentPhases[phase]; exists {
					continue
				}
			}
			if stale[namespace] == nil {
				stale[namespace] = make(map[string]struct{})
			}
			stale[namespace][phase] = struct{}{}
		}
	}
	return stale
}
