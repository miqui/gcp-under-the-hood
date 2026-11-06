#!/usr/bin/env bash
# Destroys everything this project created. Requires GCP.
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id (export TF_VAR_project_id=\$GCP_PROJECT_ID)}"
: "${TF_VAR_region:?set TF_VAR_region (export TF_VAR_region=\$GCP_REGION)}"
cd "$(dirname "$0")/../terraform"
terraform init -input=false
terraform destroy -auto-approve -input=false
echo "Done. Verify: gcloud run services list --project \"$TF_VAR_project_id\"; gcloud compute forwarding-rules list --global --project \"$TF_VAR_project_id\""
