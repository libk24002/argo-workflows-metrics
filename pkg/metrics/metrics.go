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

	// ExporterInformerSynced tracks informer sync status
	ExporterInformerSynced = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_exporter_informer_synced",
			Help: "Informer cache sync status (1=synced, 0=not synced)",
		},
		[]string{"informer"},
	)

	// ExporterLastEventTimestamp tracks last event time per informer
	ExporterLastEventTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argo_exporter_last_event_timestamp_seconds",
			Help: "Unix timestamp of the last observed event",
		},
		[]string{"informer"},
	)

	// ExporterShuttingDown indicates exporter shutdown status
	ExporterShuttingDown = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "argo_exporter_shutting_down",
			Help: "Exporter shutdown state (1=shutting down, 0=running)",
		},
	)

	// ExporterReadiness indicates readiness status
	ExporterReadiness = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "argo_exporter_ready",
			Help: "Exporter readiness status (1=ready, 0=not ready)",
		},
	)

	// ExporterLiveness indicates liveness status
	ExporterLiveness = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "argo_exporter_alive",
			Help: "Exporter liveness status (1=alive, 0=unhealthy)",
		},
	)

	// ExporterEventsTotal tracks informer event volume
	ExporterEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argo_exporter_events_total",
			Help: "Total number of informer events handled",
		},
		[]string{"informer", "event"},
	)

	// ExporterEventHandlerErrorsTotal tracks informer handler errors
	ExporterEventHandlerErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argo_exporter_event_handler_errors_total",
			Help: "Total number of informer event handler errors",
		},
		[]string{"informer", "event"},
	)

	// ExporterInformerStartErrorsTotal tracks informer startup failures
	ExporterInformerStartErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argo_exporter_informer_start_errors_total",
			Help: "Total number of informer startup errors",
		},
		[]string{"informer"},
	)
)
