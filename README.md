# Argo Workflows Metrics Exporter

Prometheus exporter for Argo Workflows metrics.

## TL;DR

`argo-workflows-metrics` is a cluster-admin-oriented observability and governance component for Argo Workflows.
It provides a reliable metrics pipeline (HA leader election, optional namespace-hash sharding, queue-based processing, periodic full reconcile) and an operational layer (recording rules, alerts, admin dashboards) for platform teams.

## 中文简介

`argo-workflows-metrics` 面向集群管理员与平台团队，定位不是“只导出指标”的 exporter，而是 Workflow 运行治理组件。

它通过 `Informer -> Workqueue -> Reconcile -> Prometheus` 的链路，提供高可用采集（Leader Election / 可选分片）、稳定处理（重试退避 + 周期全量校正）和治理消费（规则、告警、管理员看板）。

如果你在管理多团队共享的 Argo Workflows 集群，这个项目重点帮助你快速回答三个问题：
- 哪些命名空间/业务失败率在上升？
- 哪些队列在积压、时延是否恶化？
- 监控采集系统本身是否健康、是否需要扩容或治理？

## Why This Project Exists

In shared Kubernetes clusters, platform teams need a global view of workflow health and risk:
- where failures are increasing,
- where pending backlogs are building up,
- whether the monitoring pipeline itself is healthy.

This project is designed to answer those questions quickly and consistently.

## Who It Is For

- Platform/SRE teams operating multi-tenant Argo Workflows clusters
- Cluster administrators responsible for reliability, cost, and governance
- Internal workflow platform owners who need actionable global signals

## What It Provides

- Reliable collection path:
  - Informer -> Workqueue -> Worker -> Reconcile
  - Retry/backoff for transient failures
  - Periodic full reconcile to recover from missed events
- High availability and scale options:
  - Leader election mode (default HA mode)
  - Optional namespace-hash sharding mode
- Admin governance layer:
  - Recording rules for failure ratio, backlog, duration SLO proxies
  - Alerting for platform and exporter health
  - Grafana admin dashboard and panel query catalog
- Operability and cost controls:
  - Exporter self-observability (leader, queue depth, reconcile errors, sync status)
  - Cardinality budget report script and high-cardinality metric toggles

## Architecture (At a Glance)

Kubernetes API (Workflow/Pod events)
-> Shared Informers
-> Rate-limited Workqueues
-> Reconcile Workers (+ periodic full reconcile)
-> Prometheus metrics
-> Recording Rules / Alerts
-> Grafana Admin Dashboard

## 5-Minute Demo Flow

1. Open admin dashboard: check active workflows, pending backlog, failure ratio, and p95 duration.
2. Validate exporter health: ready/alive/leader status, queue depth, reconcile error rate.
3. Drill into namespace-level top risk signals to identify noisy or failing tenants.

## Features

This project focuses on cluster-level workflow governance, not only per-workflow metric export.

- Monitors Workflow CRD resources in Kubernetes
- Exposes Prometheus metrics for workflow status, duration, and node information
- Supports watching all namespaces or specific namespace
- Lightweight and efficient using Kubernetes Informer pattern
- Supports HA leader election mode for multi-replica deployment

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
kubectl apply -f deploy/pdb.yaml
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
| `argo_exporter_is_leader` | Gauge | Leader state for this replica (1/0) | - |
| `argo_exporter_leader_transitions_total` | Counter | Leader/follower transitions | `state` |
| `argo_exporter_shard_info` | Gauge | Sharding mode/index metadata | `mode`, `shard_total`, `shard_index` |
| `argo_exporter_events_total` | Counter | Informer events handled | `informer`, `event` |
| `argo_exporter_event_handler_errors_total` | Counter | Informer handler errors | `informer`, `event` |
| `argo_exporter_informer_start_errors_total` | Counter | Informer startup errors | `informer` |
| `argo_exporter_queue_depth` | Gauge | Workqueue depth by informer | `informer` |
| `argo_exporter_reconcile_duration_seconds` | Histogram | Reconcile duration | `informer`, `operation` |
| `argo_exporter_reconcile_errors_total` | Counter | Reconcile failures | `informer`, `operation` |
| `argo_exporter_full_reconcile_total` | Counter | Full reconcile runs | `informer`, `status` |

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-namespace` | "" | Namespace to watch (empty for all) |
| `-port` | 8080 | Metrics port |
| `-resync-period` | 5m | Informer resync period |
| `-startup-grace-period` | 2m | Startup grace before event staleness checks |
| `-event-stale-threshold` | 30m | Max time without workflow/pod events before readiness fails |
| `-enable-workflow-detail-metrics` | true | Enable high-cardinality per-workflow metrics |
| `-enable-pod-container-metrics` | true | Enable per-container pod metrics |
| `-worker-count` | 1 | Worker routines per informer queue |
| `-full-reconcile-period` | 10m | Periodic full reconcile interval |
| `-shard-total` | 1 | Total shard count for namespace hash sharding |
| `-shard-index` | -1 | Shard index, derived from hostname when unset |
| `-leader-elect` | true | Enable leader election for HA |
| `-leader-election-id` | argo-workflows-metrics | Leader election lease name |
| `-leader-election-namespace` | "" | Lease namespace (defaults to POD_NAMESPACE env) |
| `-leader-election-lease-duration` | 30s | Leader election lease duration |
| `-leader-election-renew-deadline` | 20s | Leader election renew deadline |
| `-leader-election-retry-period` | 5s | Leader election retry period |

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

## Validation (Phase-1 + Phase-3 baseline)

```bash
make validate-phase1
make validate-ha

# or run directly with parameters:
# ./deploy/validate-phase1.sh <namespace> <service-name> <local-port> <remote-port>

kubectl -n cnconti get pods -l app=argo-workflows-metrics
kubectl -n cnconti get deploy argo-workflows-metrics
kubectl -n cnconti get pdb argo-workflows-metrics
kubectl -n cnconti describe clusterrole argo-workflows-metrics
kubectl -n cnconti port-forward svc/argo-workflows-metrics 8080:8080
```

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | grep -E "argo_exporter_(ready|alive|is_leader|informer_synced|last_event_timestamp_seconds|events_total)"
```

```bash
# PromQL checks (run in Prometheus/Grafana Explore)
argo:workflow:active_total
argo:workflow:failed_ratio_by_namespace
argo:workflow:duration_p95_seconds_by_namespace
argo:exporter:event_rate_5m
argo:exporter:event_handler_error_rate_5m
argo:exporter:queue_depth_max
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
- no leader replicas (3m)
- leader conflict replicas > 1 (1m)
- queue depth max > 200 (10m)
- reconcile errors > 0 in 15m

Grafana admin panel query catalog:
- `deploy/grafana-admin-panels.md`

Grafana importable dashboard JSON:
- `deploy/grafana-admin-dashboard.json`

Sharding mode guide:
- `deploy/sharding-mode.md`

Cardinality budget report script:
- `deploy/cardinality-budget-report.sh`

Long-term storage integration guide:
- `deploy/long-term-storage-guide.md`

Import steps:
1. Grafana -> Dashboards -> New -> Import
2. Upload `deploy/grafana-admin-dashboard.json`
3. Select your Prometheus datasource for `DS_PROMETHEUS`

## License

MIT License
