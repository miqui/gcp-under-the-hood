#!/usr/bin/env bash
# Teardown for 06-data-tier. Requires TF_VAR_project_id and TF_VAR_region.
# Order matters:
#  1. Cloud SQL deletion protection is ON by default -> apply it OFF first,
#     otherwise destroy fails with "deletion_protection is true".
#  2. The instance must be gone before the PSA peering/range: the peering is
#     "in use" by the producer (Google tenant) VPC until the instance is deleted.
#     Terraform's dependency graph handles this (instance depends on the
#     connection). The connection uses deletion_policy=ABANDON so removing it
#     doesn't fail; the peering disappears when the VPC is deleted.
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
cd "$(dirname "$0")/../terraform"

terraform init -input=false
echo ">> Step 1: disable Cloud SQL deletion protection"
terraform apply -input=false -auto-approve -var db_deletion_protection=false \
  -target=google_sql_database_instance.pg
echo ">> Step 2: destroy everything (instance -> peering -> range -> VPC)"
terraform destroy -input=false -auto-approve -var db_deletion_protection=false
echo ">> Verify nothing is left:"
gcloud sql instances list --project "$TF_VAR_project_id" || true
gcloud compute networks peerings list --project "$TF_VAR_project_id" || true
echo "Note: the Cloud SQL instance NAME stays reserved for ~1 week (we use a random suffix)."
