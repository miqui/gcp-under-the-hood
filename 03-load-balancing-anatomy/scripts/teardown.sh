#!/usr/bin/env bash
# Destroy everything and check for orphans. Requires GCP (terraform, gcloud, TF_VAR_project_id, TF_VAR_region).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
: "${TF_VAR_project_id:=${GCP_PROJECT_ID:-}}"
: "${TF_VAR_region:=${GCP_REGION:-}}"
if [[ -z "$TF_VAR_project_id" || -z "$TF_VAR_region" ]]; then
  echo "Set TF_VAR_project_id and TF_VAR_region (or GCP_PROJECT_ID / GCP_REGION)." >&2
  exit 1
fi
export TF_VAR_project_id TF_VAR_region

cd "$ROOT/terraform"
terraform destroy -auto-approve

echo "== Orphan check (project=$TF_VAR_project_id) =="
orphans=0
check() {
  local label="$1"; shift
  local out
  out="$("$@" --project="$TF_VAR_project_id" --filter="name~^lbanat" --format="value(name)" 2>/dev/null || true)"
  if [[ -n "$out" ]]; then
    echo "ORPHAN $label: $out"
    orphans=1
  else
    echo "ok: no $label"
  fi
}
check "forwarding rules" gcloud compute forwarding-rules list
check "backend services" gcloud compute backend-services list
check "instance groups" gcloud compute instance-groups managed list
check "instances" gcloud compute instances list
check "addresses" gcloud compute addresses list
check "health checks" gcloud compute health-checks list
check "firewall rules" gcloud compute firewall-rules list
check "networks" gcloud compute networks list
check "routers" gcloud compute routers list

if [[ $orphans -ne 0 ]]; then
  echo "Orphans found; delete them manually." >&2
  exit 2
fi
echo "Clean. (Artifact Registry repo is destroyed by terraform; images go with it.)"
