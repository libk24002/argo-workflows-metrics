# Phase-3 Implementation Checklist

## Completed in this iteration

- [x] Enable HA deployment baseline (2 replicas, rolling update, anti-affinity, topology spread)
- [x] Add leader election in exporter runtime
- [x] Make readiness leader-aware to avoid duplicate scraping
- [x] Add lease RBAC permissions for leader election
- [x] Add PodDisruptionBudget (`minAvailable: 1`)
- [x] Add leader metrics (`argo_exporter_is_leader`, `argo_exporter_leader_transitions_total`)
- [x] Add no-leader and leader-conflict alerts
- [x] Add cardinality control flags for detailed metrics and pod container metrics
- [x] Implement queue-based informer processing with retry/backoff
- [x] Add periodic full reconcile loop to recover from missed events
- [x] Add queue depth and reconcile health metrics/alerts
- [x] Add optional sharding mode (namespace/hash) as alternative to single leader
- [x] Add metric cardinality budget report (series count by metric/label)
- [x] Add long-term storage integration guide (Thanos/Mimir) and retention policy

## Next items

- [ ] Add StatefulSet example manifests for auto-derived shard index
- [ ] Add query parity smoke checks between local and long-term backends
