#!/usr/bin/env bash
# Destroys everything created by this project. Requires GCP (terraform destroy).
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
cd "$(dirname "$0")/../terraform"
terraform destroy -auto-approve
echo "Done. Note: WIF pools are soft-deleted for 30 days (ID can't be reused; random suffix avoids clashes)."
