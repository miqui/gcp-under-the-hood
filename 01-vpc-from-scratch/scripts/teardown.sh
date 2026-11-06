#!/usr/bin/env bash
# Destroy everything and check for orphans. Requires GCP. Uses TF_VAR_project_id / TF_VAR_region.
set -euo pipefail
: "${TF_VAR_project_id:?set TF_VAR_project_id}"
: "${TF_VAR_region:?set TF_VAR_region}"
P="$TF_VAR_project_id"
cd "$(dirname "$0")/../terraform"
mkdir -p ../go/bin
[ -f ../go/bin/probe ] || : > ../go/bin/probe # placeholder so destroy can evaluate
terraform destroy -auto-approve
echo "== orphan checks (all should print nothing) =="
rc=0
chk() {
  out=$("$@" 2>&1 || true)
  if [ -n "$out" ]; then
    echo "ORPHAN: $*"
    echo "$out"
    rc=1
  fi
}
chk gcloud compute instances list --project "$P" --filter="name~'^(a-|b-|c-)'" --format="value(name)"
chk gcloud compute networks list --project "$P" --filter="name~'^vpc-[abc]$'" --format="value(name)"
chk gcloud compute routers list --project "$P" --filter="name=a-router" --format="value(name)"
chk gcloud compute addresses list --project "$P" --format="value(name)"
chk gcloud compute firewall-rules list --project "$P" --filter="network~'vpc-[abc]$'" --format="value(name)"
chk gcloud storage buckets list --project "$P" --filter="name~vpc01-bin" --format="value(name)"
if [ "$rc" -eq 0 ]; then echo clean; else
  echo "orphans found"
  exit 1
fi
