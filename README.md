# Argo Workflows Metrics Exporter

Prometheus exporter for Argo Workflows metrics.

## Features

- Monitors Workflow CRD resources in Kubernetes
- Exposes Prometheus metrics for workflow status, duration, and node information
- Supports watching all namespaces or specific namespace
- Lightweight and efficient using Kubernetes Informer pattern

## Quick Start

### Docker Deployment

```bash
# Pull image
docker pull ghcr.io/libk24002/argo-workflows-metrics:latest

# Run container
docker run -d \
  --name argo-workflows-metrics \
  -p 8080:8080 \
  ghcr.io/libk24002/argo-workflows-metrics:latest
```

### Kubernetes Deployment

```bash
# Apply RBAC
kubectl apply -f deploy/rbac.yaml

# Deploy
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```

## Metrics

| Metric Name | Type | Description | Labels |
|--------|-------------|--------|
| `argo_workflow_status_total` | Gauge | Total number of workflows by status phase | `namespace`, `phase` |
| `argo_workflow_status_phase` | Gauge | Current phase of workflow | `namespace`, `name`, `phase` |
| `argo_workflow_count` | Gauge | Total number of workflows | `namespace` |
| `argo_workflow_duration_seconds` | Gauge | Workflow execution duration | `namespace`, `name`, `phase` |
| `argo_workflow_created_time` | Gauge | Workflow creation timestamp | `namespace`, `name` |
| `argo_workflow_started_time` | Gauge | Workflow start timestamp | `namespace`, `name` |
| `argo_workflow_finished_time` | Gauge | Workflow finish timestamp | `namespace`, `name` |
| `argo_workflow_node_total` | Gauge | Total number of nodes | `namespace`, `name` |
| `argo_workflow_node_phase` | Gauge | Nodes by phase | `namespace`, `name`, `phase` |
| `argo_workflow_info` | Gauge | Workflow metadata | `namespace`, `name`, `uid`, `service_account`, `priority` |

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-namespace` | "" | Namespace to watch (empty for all) |
| `-port` | 8080 | Metrics port |
| `-resync-period` | 5m | Informer resync period |

## Development

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test
```

## Accessing Metrics

```bash
curl http://localhost:8080/metrics
curl http://localhost:8080/healthz
```

## License

MIT License
