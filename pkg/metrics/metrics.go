package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WorkflowStatusTotal tracks the total number of workflows by status phase
	WorkflowStatusTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_status_total",
			Help: "Total number of workflows by status phase",
		},
		[]string{"namespace", "phase"},
	)

	// WorkflowStatusPhase tracks the current phase of each workflow
	WorkflowStatusPhase = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_status_phase",
			Help: "Current phase of workflow (1 for current phase, 0 otherwise)",
		},
		[]string{"namespace", "name", "phase"},
	)

	// WorkflowCount tracks the total number of workflows in the cluster
	WorkflowCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_count",
			Help: "Total number of workflows in the cluster",
		},
		[]string{"namespace"},
	)

	// WorkflowDurationSeconds tracks workflow execution duration
	WorkflowDurationSeconds = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_duration_seconds",
			Help: "Workflow execution duration in seconds",
		},
		[]string{"namespace", "name", "phase"},
	)

	// WorkflowCreatedTime tracks workflow creation timestamp
	WorkflowCreatedTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_created_time",
			Help: "Workflow creation timestamp",
		},
		[]string{"namespace", "name"},
	)

	// WorkflowStartedTime tracks workflow start timestamp
	WorkflowStartedTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_started_time",
			Help: "Workflow start timestamp",
		},
		[]string{"namespace", "name"},
	)

	// WorkflowFinishedTime tracks workflow finish timestamp
	WorkflowFinishedTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_finished_time",
			Help: "Workflow finish timestamp",
		},
		[]string{"namespace", "name"},
	)

	// WorkflowNodeTotal tracks total number of nodes in workflow
	WorkflowNodeTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_node_total",
			Help: "Total number of nodes in workflow",
		},
		[]string{"namespace", "name"},
	)

	// WorkflowNodePhase tracks node status distribution
	WorkflowNodePhase = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_node_phase",
			Help: "Number of nodes by phase in workflow",
		},
		[]string{"namespace", "name", "phase"},
	)

	// WorkflowInfo provides workflow metadata
	WorkflowInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_info",
			Help: "Workflow metadata information",
		},
		[]string{"namespace", "name", "uid", "service_account", "priority"},
	)

	// ContainerCPUUsageSeconds tracks container CPU usage in seconds
	ContainerCPUUsageSeconds = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argo_workflow_container_cpu_usage_seconds_total",
			Help: "Total CPU time consumed by containers in seconds",
		},
		[]string{"namespace", "workflow_name", "node_name", "container_name"},
	)

	// ContainerMemoryUsageBytes tracks container memory usage in bytes
	ContainerMemoryUsageBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_workflow_container_memory_usage_bytes",
			Help: "Memory usage by containers in bytes",
		},
		[]string{"namespace", "workflow_name", "node_name", "container_name"},
	)
)
