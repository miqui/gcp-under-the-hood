#!/usr/bin/env bash
# Destroys everything and checks for orphans. Requires GCP_PROJECT_ID/GCP_REGION
# (or TF_VAR_project_id / TF_VAR_region).
set -euo pipefail
cd "$(dirname "$0")/../terraform"
export TF_VAR_project_id="${TF_VAR_project_id:-${GCP_PROJECT_ID:?set GCP_PROJECT_ID}}"
export TF_VAR_region="${TF_VAR_region:-${GCP_REGION:?set GCP_REGION}}"

terraform init -input=false
terraform destroy -auto-approve -input=false

echo "== orphan checks =="
p="$TF_VAR_project_id"
for cmd in \
  "gcloud compute vpn-tunnels list" \
  "gcloud compute vpn-gateways list" \
  "gcloud compute routers list" \
  "gcloud compute instances list" \
  "gcloud compute addresses list" \
  "gcloud compute networks list" \
  "gcloud dns managed-zones list" \
  "gcloud dns policies list"; do
  out=$($cmd --project="$p" --filter="name~^hyb-" --format='value(name)' 2>&1 || true)
  if [ -n "$out" ]; then
    echo "ORPHAN from [$cmd]:"
    echo "$out"
  fi
done
echo "teardown check complete"
