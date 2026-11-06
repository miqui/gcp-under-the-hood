#!/usr/bin/env bash
# Cost-safe teardown: terraform destroy, then check for orphaned Pub/Sub resources.
set -euo pipefail

: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/terraform"

terraform init -input=false
terraform destroy -auto-approve -input=false

echo "== Orphan checks (anything listed with prefix psds- is left over) =="
left=0
for kind in topics subscriptions; do
  out="$(gcloud pubsub "$kind" list --project "$TF_VAR_project_id" --format='value(name)' 2>/dev/null | grep '/psds-' || true)"
  if [ -n "$out" ]; then
    echo "ORPHAN $kind:"
    echo "$out"
    left=1
  fi
done
if [ "$left" -eq 0 ]; then
  echo "No orphaned psds- topics/subscriptions."
else
  echo "Delete manually: gcloud pubsub <topics|subscriptions> delete <name>"
  exit 1
fi
