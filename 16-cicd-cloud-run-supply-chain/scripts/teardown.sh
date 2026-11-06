#!/usr/bin/env bash
# Cost-safe teardown: terraform destroy, then check for orphans. Requires GCP.
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
: "${TF_VAR_github_repo:?set TF_VAR_github_repo}"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../terraform" && pwd)"
cd "$DIR"
terraform destroy -auto-approve

P="$TF_VAR_project_id"
R="$TF_VAR_region"
echo "== orphan checks (expect empty) =="
gcloud run services list --project "$P" --region "$R" --format='value(name)' || true
gcloud artifacts repositories list --project "$P" --location "$R" --format='value(name)' || true
gcloud deploy delivery-pipelines list --project "$P" --region "$R" --format='value(name)' || true
gcloud iam workload-identity-pools list --project "$P" --location global --format='value(name)' || true
echo "Note: KMS key versions are scheduled for destruction (24h); WIF pools are soft-deleted for 30 days."
echo "Cloud Build source bucket gs://${P}_cloudbuild may remain; delete manually if unwanted."
