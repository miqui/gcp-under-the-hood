#!/usr/bin/env bash
# Requires GCP. Delete k8s LB resources first (they create GCP objects Terraform doesn't track), then destroy infra.
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
cd "$(dirname "$0")/.."
kubectl delete -f k8s/service-ingress.yaml --ignore-not-found --wait=true || true
kubectl delete ns demo --ignore-not-found || true
sleep 30 # let the controller remove forwarding rules / NEGs
(cd terraform && terraform destroy -auto-approve)
echo "Done. Verify: gcloud container clusters list --project $TF_VAR_project_id"
