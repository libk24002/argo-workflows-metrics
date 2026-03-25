# Grafana Admin Panels (Cluster Admin View)

This panel list is aligned with `deploy/prometheusrule.yaml` and can be pasted directly into Grafana panel queries.

## Core Overview

| Panel | Type | PromQL | Unit |
|------|------|--------|------|
| Active Workflows | Stat | `argo:workflow:active_total` | none |
| Pending Backlog | Stat | `argo:workflow:pending_backlog_cluster` | none |
| Failure Ratio | Stat | `argo:workflow:failure_ratio_cluster` | percent (0-1) |
| Workflow P95 Duration | Stat | `argo:workflow:duration_p95_seconds_cluster` | seconds |
| Workflows by Phase | Time series (stack) | `argo:workflow:total_by_phase` | none |

## Namespace Governance

| Panel | Type | PromQL | Unit |
|------|------|--------|------|
| Top 10 Running Namespaces | Bar gauge | `topk(10, argo:workflow:running_by_namespace)` | none |
| Top 10 Pending Namespaces | Bar gauge | `topk(10, argo:workflow:pending_by_namespace)` | none |
| Top 10 Failure Ratio Namespaces | Bar gauge | `topk(10, argo:workflow:failed_ratio_by_namespace)` | percent (0-1) |
| Top 10 P95 Duration Namespaces | Bar gauge | `topk(10, argo:workflow:duration_p95_seconds_by_namespace)` | seconds |

## Exporter Health (Control Plane)

| Panel | Type | PromQL | Unit |
|------|------|--------|------|
| Exporter Ready | Stat | `max(argo_exporter_ready)` | none |
| Exporter Alive | Stat | `max(argo_exporter_alive)` | none |
| Exporter Shutting Down | Stat | `max(argo_exporter_shutting_down)` | none |
| Informer Synced | Time series | `argo_exporter_informer_synced` | none |
| Last Event Age | Time series | `time() - argo_exporter_last_event_timestamp_seconds` | seconds |
| Event Rate (5m) | Time series | `argo:exporter:event_rate_5m` | ops |
| Event Handler Error Rate (5m) | Time series | `argo:exporter:event_handler_error_rate_5m` | ops |

## Suggested Thresholds

- Failure ratio warning: `> 0.20` for `10m`
- Pending backlog warning: `> 100` for `15m`
- P95 duration warning: `> 3600` seconds for `15m`
- Informer unsynced warning: `== 0` for `10m`
- No recent events warning: `time() - argo_exporter_last_event_timestamp_seconds > 3600` for `15m`

## Dashboard Layout Suggestion

1. Row 1: Active Workflows / Pending Backlog / Failure Ratio / P95 Duration
2. Row 2: Workflows by Phase + Running/Pending namespace top lists
3. Row 3: Failure ratio + P95 by namespace top lists
4. Row 4: Exporter readiness/liveness + informer/event diagnostics
