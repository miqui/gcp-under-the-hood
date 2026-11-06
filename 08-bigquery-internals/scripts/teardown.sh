#!/usr/bin/env bash
# Cost-safe teardown: destroy Terraform resources, then check for orphans.
# Requires GCP (terraform + bq CLI). Env: TF_VAR_project_id, TF_VAR_region.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
DATASET="${BQ_DATASET:-bq_internals_lab}"

cd "$ROOT/terraform"
terraform init -input=false
terraform destroy -auto-approve -input=false

echo "== orphan check: dataset ${DATASET} =="
if command -v bq >/dev/null 2>&1; then
  if bq --project_id="$TF_VAR_project_id" ls --format=json 2>/dev/null | grep -q "\"datasetId\": *\"${DATASET}\""; then
    echo "WARNING: dataset ${DATASET} still exists. Remove with:"
    echo "  bq rm -r -f -d ${TF_VAR_project_id}:${DATASET}"
    exit 1
  fi
  echo "OK: dataset gone."
else
  echo "bq CLI not found; verify manually in the console that ${DATASET} is gone."
fi

echo "== reminder: if you created reservations/commitments manually, delete them =="
echo "  bq ls --reservation --project_id=$TF_VAR_project_id --location=US"
