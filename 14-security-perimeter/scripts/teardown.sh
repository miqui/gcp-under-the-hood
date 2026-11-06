#!/usr/bin/env bash
# Cost-safe teardown for 14-security-perimeter.
# NOTE: KMS key rings/keys cannot be deleted. `terraform destroy` removes the key from
# state; key VERSIONS are scheduled for destruction (after key_destroy_scheduled_duration,
# default 24h). The empty ring/key remain (free) and the names stay reserved.
set -euo pipefail

: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../terraform" && pwd)"

cd "$DIR"
terraform destroy -auto-approve

echo "== orphan checks =="
P="$TF_VAR_project_id"
echo "-- buckets:";   gcloud storage buckets list --project "$P" --filter="name~secperim" --format="value(name)" || true
echo "-- secrets:";   gcloud secrets list --project "$P" --filter="name~secperim" --format="value(name)" || true
echo "-- armor:";     gcloud compute security-policies list --project "$P" --filter="name~secperim" --format="value(name)" || true
echo "-- firewalls:"; gcloud compute firewall-rules list --project "$P" --filter="name~secperim" --format="value(name)" || true
echo "-- kms key versions (expect DESTROY_SCHEDULED/DESTROYED):"
gcloud kms keys versions list --key "secperim-kek" --keyring "secperim-ring" --location "$TF_VAR_region" --project "$P" --format="table(name.basename(),state)" || true
echo "-- vpc-sc perimeters (org-level; check manually if enable_vpc_sc was true):"
echo "   gcloud access-context-manager perimeters list --policy=<POLICY_ID>"
