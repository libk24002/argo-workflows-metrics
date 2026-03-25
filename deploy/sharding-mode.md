# Namespace Hash Sharding Mode

Sharding mode lets multiple replicas split workload by namespace hash instead of leader election.

## Runtime flags

- `-shard-total=<N>` total shard count
- `-shard-index=<i>` shard index in `[0, N-1]`
- `-leader-elect=false` must be disabled in sharding mode

If `-shard-index` is not set (`-1`), exporter derives it from the pod hostname suffix
(recommended with StatefulSet pod names like `argo-metrics-0`, `argo-metrics-1`).

## Example (2 shards)

Shard 0:

```bash
./bin/exporter \
  -leader-elect=false \
  -shard-total=2 \
  -shard-index=0
```

Shard 1:

```bash
./bin/exporter \
  -leader-elect=false \
  -shard-total=2 \
  -shard-index=1
```

## Verification

- `argo_exporter_shard_info{mode="shard"} == 1`
- `sum(argo_exporter_shard_info{mode="shard"}) == <replica count>`
- per-instance `argo_exporter_queue_depth` remains balanced over time

## Notes

- Keep shard count fixed during peak hours; changing shard count rebalances namespaces.
- Prefer StatefulSet for deterministic shard indices.
- Use this mode only when cluster scale exceeds single-leader throughput.
