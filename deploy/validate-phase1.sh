#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-cnconti}"
SERVICE_NAME="${2:-argo-workflows-metrics}"
LOCAL_PORT="${3:-18080}"
REMOTE_PORT="${4:-8080}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

wait_http() {
  local url="$1"
  local retries=30
  while [[ "$retries" -gt 0 ]]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    retries=$((retries - 1))
    sleep 1
  done
  return 1
}

require_cmd kubectl
require_cmd curl

printf 'Namespace: %s\n' "$NAMESPACE"
printf 'Service: %s\n' "$SERVICE_NAME"

kubectl -n "$NAMESPACE" get deploy "$SERVICE_NAME"
kubectl -n "$NAMESPACE" get svc "$SERVICE_NAME"
kubectl -n "$NAMESPACE" wait --for=condition=Available "deployment/$SERVICE_NAME" --timeout=180s

replicas="$(kubectl -n "$NAMESPACE" get deploy "$SERVICE_NAME" -o jsonpath='{.spec.replicas}')"
if [[ "$replicas" -lt 2 ]]; then
  printf 'expected deployment replicas >= 2 for HA, got %s\n' "$replicas" >&2
  exit 1
fi

kubectl get clusterrole argo-workflows-metrics >/dev/null
kubectl -n "$NAMESPACE" get prometheusrule "$SERVICE_NAME" >/dev/null
kubectl -n "$NAMESPACE" get pdb "$SERVICE_NAME" >/dev/null

pf_log="/tmp/${SERVICE_NAME}-port-forward.log"
kubectl -n "$NAMESPACE" port-forward "svc/$SERVICE_NAME" "${LOCAL_PORT}:${REMOTE_PORT}" >"$pf_log" 2>&1 &
pf_pid=$!

cleanup() {
  if kill -0 "$pf_pid" >/dev/null 2>&1; then
    kill "$pf_pid" >/dev/null 2>&1 || true
    wait "$pf_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

base_url="http://127.0.0.1:${LOCAL_PORT}"
wait_http "$base_url/healthz"

healthz_resp="$(curl -fsS "$base_url/healthz")"
readyz_resp="$(curl -fsS "$base_url/readyz")"
metrics_resp="$(curl -fsS "$base_url/metrics")"

printf '\n[healthz]\n%s\n' "$healthz_resp"
printf '\n[readyz]\n%s\n' "$readyz_resp"

if [[ "$metrics_resp" != *"argo_exporter_ready"* ]]; then
  printf 'missing metric: argo_exporter_ready\n' >&2
  exit 1
fi
if [[ "$metrics_resp" != *"argo_exporter_alive"* ]]; then
  printf 'missing metric: argo_exporter_alive\n' >&2
  exit 1
fi
if [[ "$metrics_resp" != *"argo_exporter_informer_synced"* ]]; then
  printf 'missing metric: argo_exporter_informer_synced\n' >&2
  exit 1
fi
if [[ "$metrics_resp" != *"argo_exporter_events_total"* ]]; then
  printf 'missing metric: argo_exporter_events_total\n' >&2
  exit 1
fi
if [[ "$metrics_resp" != *"argo_exporter_is_leader"* ]]; then
  printf 'missing metric: argo_exporter_is_leader\n' >&2
  exit 1
fi
if [[ "$metrics_resp" != *"argo_exporter_queue_depth"* ]]; then
  printf 'missing metric: argo_exporter_queue_depth\n' >&2
  exit 1
fi

printf '\nPhase-1/3 baseline checks passed.\n\n'
printf 'PromQL checks:\n'
printf '%s\n' '  argo:workflow:active_total'
printf '%s\n' '  argo:workflow:failed_ratio_by_namespace'
printf '%s\n' '  argo:workflow:duration_p95_seconds_by_namespace'
printf '%s\n' '  argo:exporter:event_rate_5m'
printf '%s\n' '  argo:exporter:event_handler_error_rate_5m'
