# gcp-under-the-hood

Hands-on GCP labs that teach how GKE, Cloud Run, VPC, IAM, Cloud SQL, Cloud Storage and Load Balancing work under the hood — Terraform + Go, predict-then-verify.

## Purpose
Each project provisions real infrastructure with Terraform and runs small Go workloads against it, so you can observe how the platform actually behaves instead of trusting the docs.

## Suggested learning order
Every project is standalone (no cross-project dependencies), but this order builds concepts progressively:

1. [`01-vpc-from-scratch`](01-vpc-from-scratch) — custom VPC, subnets, routes, firewall rules, NAT. Foundation for networking concepts used later.
2. [`02-iam-deep-dive`](02-iam-deep-dive) — service accounts, roles, policy inheritance, impersonation, workload identity basics.
3. [`03-load-balancing-anatomy`](03-load-balancing-anatomy) — forwarding rules, proxies, URL maps, backend services, health checks.
4. [`04-cloud-run-internals`](04-cloud-run-internals) — revisions, scaling, concurrency, cold starts, ingress and identity.
5. [`05-gke-networking-identity`](05-gke-networking-identity) — pod/service networking, Workload Identity, network policy.
6. [`06-data-tier`](06-data-tier) — Cloud SQL (private connectivity) and Cloud Storage access patterns.

Layout per project: `terraform/` (infra) and `go/` (workloads).

## Conventions
- **Predict-then-verify:** write down what you expect before running each experiment, then check against the output.
- **Break it on purpose:** each lab includes deliberate misconfigurations to see how failures present.
- **Teardown scripts:** each project ships a teardown script. Always run it when finished.
- **Cost warning:** these labs create billable resources (GKE clusters, load balancers, Cloud NAT, Cloud SQL). **Run the teardown script after every session** and check the Console for leftovers; consider a budget alert on your project.

## Prerequisites
- A GCP project with billing enabled (use a disposable one)
- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
- [Go](https://go.dev/dl/) (version per each project's `go.mod`)
- [gcloud CLI](https://cloud.google.com/sdk/docs/install)

## Environment variables
No project details are hardcoded. Set `GCP_PROJECT_ID`, `GCP_REGION`, `GCP_ZONE` and mirror them as `TF_VAR_project_id`, `TF_VAR_region`, `TF_VAR_zone`. Authenticate with `gcloud auth application-default login` or `GOOGLE_APPLICATION_CREDENTIALS`. Credentials are never committed. See [docs/env-vars.md](docs/env-vars.md).

## CI
The `validate` workflow runs `terraform fmt -check`, `init -backend=false`, `validate`, plus `go vet` and `go test ./...` per project. No GCP credentials are used.

## License
[MIT](LICENSE) © Miguel Quintero
