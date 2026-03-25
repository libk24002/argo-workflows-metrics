# Long-Term Storage Integration Guide

This guide provides a baseline for storing Argo workflow metrics for 90+ days.

## Option A: Thanos (recommended for Prometheus Operator)

1. Enable Prometheus external labels (`cluster`, `environment`).
2. Configure object storage bucket (S3/OSS/GCS) for Thanos blocks.
3. Deploy Thanos Sidecar with Prometheus and enable block shipping.
4. Deploy Thanos Store Gateway + Query + Compactor.
5. Point Grafana datasource to Thanos Query endpoint.

Suggested retention:
- local Prometheus: 15d
- object storage via Thanos: 180d+

## Option B: Grafana Mimir (multi-tenant)

1. Deploy Mimir with object storage backend.
2. Configure Prometheus `remote_write` to Mimir distributor.
3. Add tenant routing labels (`cluster`, `team`) in write relabels.
4. Keep local Prometheus retention short (7-15d).
5. Use Mimir query frontend as Grafana datasource.

## Recording Rules Migration

- Keep fast operational rules in local Prometheus (`5m` windows).
- Mirror governance/capacity rules to long-term backend (`1h/1d` windows).
- Keep rule names stable (`argo:*`) for dashboard compatibility.

## Operational Checks

- Query parity check: compare same query across local and long-term backends.
- Backfill check: verify historical data appears after 24h.
- Cost check: track ingestion rate, active series, object storage growth.

## Recommended Labels for Query Hygiene

- required: `cluster`, `environment`, `namespace`
- optional: `team`, `template`
- avoid in long-term dashboards: `uid`, `pod`, `container`
