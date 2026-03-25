#!/usr/bin/env bash
set -euo pipefail

PROMETHEUS_URL="${1:-http://localhost:9090}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd python3

payload="$(curl -fsS "${PROMETHEUS_URL}/api/v1/status/tsdb")"

python3 - <<'PY' "$payload"
import json
import sys

budgets = {
    "argo_workflow_status_total": 5000,
    "argo_workflow_status_phase": 80000,
    "argo_workflow_count": 5000,
    "argo_workflow_duration_seconds": 80000,
    "argo_workflow_node_phase": 100000,
    "argo_workflow_info": 80000,
    "argo_workflow_container_cpu_usage_seconds_total": 30000,
    "argo_workflow_container_memory_usage_bytes": 30000,
}

label_budgets = {
    "namespace": 1000,
    "name": 60000,
    "uid": 70000,
    "workflow_name": 60000,
    "node_name": 60000,
    "container_name": 20000,
}

doc = json.loads(sys.argv[1])
if doc.get("status") != "success":
    print("failed to fetch TSDB status")
    sys.exit(2)

data = doc.get("data", {})
series_by_metric = data.get("seriesCountByMetricName", {})
label_values = data.get("labelValueCountByLabelName", {})

print("Metric cardinality budget report")
print("=" * 72)
print(f"{'metric':52} {'series':>8} {'budget':>8} {'status':>8}")

has_violation = False
for metric, budget in budgets.items():
    count = int(series_by_metric.get(metric, 0))
    status = "OK" if count <= budget else "EXCEED"
    if status != "OK":
        has_violation = True
    print(f"{metric:52} {count:8d} {budget:8d} {status:>8}")

print("\nLabel cardinality budget report")
print("=" * 72)
print(f"{'label':24} {'values':>8} {'budget':>8} {'status':>8}")
for label, budget in label_budgets.items():
    count = int(label_values.get(label, 0))
    status = "OK" if count <= budget else "EXCEED"
    if status != "OK":
        has_violation = True
    print(f"{label:24} {count:8d} {budget:8d} {status:>8}")

if has_violation:
    print("\nResult: budget violations detected")
    sys.exit(1)

print("\nResult: all budgets are within limits")
PY
