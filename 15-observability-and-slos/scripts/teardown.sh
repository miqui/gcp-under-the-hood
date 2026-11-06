#!/usr/bin/env bash
# Cost-safe teardown: terraform destroy, then check for orphans.
# Requires TF_VAR_project_id and TF_VAR_region (GCP_PROJECT_ID / GCP_REGION fallback).
set -euo pipefail

export TF_VAR_project_id="${TF_VAR_project_id:-${GCP_PROJECT_ID:-}}"
export TF_VAR_region="${TF_VAR_region:-${GCP_REGION:-}}"
: "${TF_VAR_project_id:?set TF_VAR_project_id or GCP_PROJECT_ID}"
: "${TF_VAR_region:?set TF_VAR_region or GCP_REGION}"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../terraform" && pwd)"
cd "$DIR"
terraform init -input=false
terraform destroy -auto-approve -input=false

echo "== Orphan checks (project ${TF_VAR_project_id}) =="
orphans=0
check() {
  local label="$1"; shift
  local out
  out="$("$@" 2>/dev/null || true)"
  if [ -n "$out" ]; then
    echo "ORPHAN? ${label}:"; echo "$out"; orphans=1
  else
    echo "ok: no ${label}"
  fi
}
check "Cloud Run service slo-demo" gcloud run services list --project "$TF_VAR_project_id" --region "$TF_VAR_region" --filter="metadata.name=slo-demo" --format="value(metadata.name)"
check "log sink slo-demo-errors-sink" gcloud logging sinks list --project "$TF_VAR_project_id" --filter="name=slo-demo-errors-sink" --format="value(name)"
check "log metrics slo_demo_*" gcloud logging metrics list --project "$TF_VAR_project_id" --filter="name:slo_demo_" --format="value(name)"
check "alert policies" gcloud alpha monitoring policies list --project "$TF_VAR_project_id" --filter="displayName:SLO OR displayName:Injected" --format="value(displayName)"
check "dashboard SLO demo" gcloud monitoring dashboards list --project "$TF_VAR_project_id" --filter="displayName='SLO demo'" --format="value(displayName)"

# Log buckets are soft-deleted for 7 days by Cloud Logging; that is expected and free.
echo "Note: bucket slo-demo-errors may show as pending deletion for 7 days (no cost)."
exit "$orphans"
