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
kubectl apply -f deploy/prometheusrule.yaml
```

## Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
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
| `argo_exporter_informer_synced` | Gauge | Informer cache sync status (1/0) | `informer` |
| `argo_exporter_last_event_timestamp_seconds` | Gauge | Last observed event timestamp | `informer` |
| `argo_exporter_shutting_down` | Gauge | Exporter shutdown state (1/0) | - |
| `argo_exporter_ready` | Gauge | Exporter readiness state (1/0) | - |
| `argo_exporter_alive` | Gauge | Exporter liveness state (1/0) | - |
| `argo_exporter_events_total` | Counter | Informer events handled | `informer`, `event` |
| `argo_exporter_event_handler_errors_total` | Counter | Informer handler errors | `informer`, `event` |
| `argo_exporter_informer_start_errors_total` | Counter | Informer startup errors | `informer` |

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-namespace` | "" | Namespace to watch (empty for all) |
| `-port` | 8080 | Metrics port |
| `-resync-period` | 5m | Informer resync period |
| `-startup-grace-period` | 2m | Startup grace before event staleness checks |
| `-event-stale-threshold` | 30m | Max time without workflow/pod events before readiness fails |

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
curl http://localhost:8080/readyz
```

## Phase-1 Validation

```bash
make validate-phase1

# or run directly with parameters:
# ./deploy/validate-phase1.sh <namespace> <service-name> <local-port> <remote-port>

kubectl -n cnconti get pods -l app=argo-workflows-metrics
kubectl -n cnconti get deploy argo-workflows-metrics
kubectl -n cnconti describe clusterrole argo-workflows-metrics
kubectl -n cnconti port-forward svc/argo-workflows-metrics 8080:8080
```

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | grep -E "argo_exporter_(ready|alive|informer_synced|last_event_timestamp_seconds|events_total)"
```

```bash
# PromQL checks (run in Prometheus/Grafana Explore)
argo:workflow:active_total
argo:workflow:failed_ratio_by_namespace
argo:workflow:duration_p95_seconds_by_namespace
argo:exporter:event_rate_5m
argo:exporter:event_handler_error_rate_5m
```

```bash
# Admin-level recording metrics
argo:workflow:total_by_phase
argo:workflow:pending_backlog_cluster
argo:workflow:failure_ratio_cluster
argo:workflow:duration_p95_seconds_cluster
```

Admin alerts in `deploy/prometheusrule.yaml` use default thresholds:
- failure ratio > 20% (10m)
- pending backlog > 100 (15m)
- p95 duration > 3600s (15m)

Grafana admin panel query catalog:
- `deploy/grafana-admin-panels.md`

Grafana importable dashboard JSON:
- `deploy/grafana-admin-dashboard.json`

Import steps:
1. Grafana -> Dashboards -> New -> Import
2. Upload `deploy/grafana-admin-dashboard.json`
3. Select your Prometheus datasource for `DS_PROMETHEUS`

## License

MIT License
