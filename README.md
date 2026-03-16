# Argo Workflows Metrics Exporter

Prometheus exporter for Argo Workflows metrics.

## Features

- Monitors Workflow CRD resources in Kubernetes
- Exposes Prometheus metrics for workflow status, duration, and node information
- Supports watching all namespaces or specific namespace
- Lightweight and efficient using Kubernetes Informer pattern

## Metrics

### Phase 1.1 - Core Workflow Metrics

| Metric Name | Type | Description | Labels |
|--------|-------------|--------|
| `argo_workflow_status_total` | Gauge | Total number of workflows by status phase | `namespace`, `phase` |
| `argo_workflow_status_phase` | Gauge | Current phase of workflow (1 for current phase, 0 otherwise) | `namespace`, `name`, `phase` |
| `argo_workflow_count` | Gauge | Total number of workflows in the cluster | `namespace` |
| `argo_workflow_duration_seconds` | Gauge | Workflow execution duration in seconds | `namespace`, `name`, `phase` |
| `argo_workflow_created_time` | Gauge | Workflow creation timestamp | `namespace`, `name` |
| `argo_workflow_started_time` | Gauge | Workflow start timestamp | `namespace`, `name` |
| `argo_workflow_finished_time` | Gauge | Workflow finish timestamp | `namespace`, `name` |
| `argo_workflow_node_total` | Gauge | Total number of nodes in workflow | `namespace`, `name` |
| `argo_workflow_node_phase` | Gauge | Number of nodes by phase in workflow | `namespace`, `name`, `phase` |
| `argo_workflow_info` | Gauge | Workflow metadata information | `namespace`, `name`, `uid`, `service_account`, `priority` |

## Prerequisites

- Kubernetes cluster with Argo Workflows installed
- Go 1.22+ (for building)
- Docker (for containerization)

## Building

```bash
# Build binary
make build

# Build Docker image
make docker-build
```

## Deployment

### 1. Apply RBAC permissions

```bash
kubectl apply -f deploy/rbac.yaml
```

### 2. Deploy the exporter

```bash
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```

### 3. (Optional) Create ServiceMonitor for Prometheus Operator

```bash
kubectl apply -f deploy/servicemonitor.yaml
```

## Configuration

Command-line flags:

- `-kubeconfig`: Path to kubeconfig file (optional, uses in-cluster config if not provided)
- `-namespace`: Namespace to watch (empty for all namespaces)
- `-port`: Port to expose metrics on (default: 8080)
- `-resync-period`: Resync period for informer (default: 5m)
- `-v`: Log verbosity level

## Usage

### Running locally

```bash
# Watch all namespaces
go run ./cmd/exporter -kubeconfig ~/.kube/config

# Watch specific namespace
go run ./cmd/exporter -kubeconfig ~/.kube/config -namespace=argo
```

### Accessing metrics

```bash
# Metrics endpoint
curl http://localhost:8080/metrics

# Health check
curl http://localhost:8080/healthz
```

## Prometheus Configuration

If not using Prometheus Operator, add the following scrape config:

```yaml
scrape_configs:
  - job_name: 'argo-workflows-metrics'
    kubernetes_sd_configs:
    - role: service
    relabel_configs:
    - source_labels: [__meta_kubernetes_service_label_app]
      regex: argo-workflows-metrics
      action: keep
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_service_name]
      target_label: service
```

## License

MIT License - see LICENSE file for details
