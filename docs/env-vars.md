# Environment variables

| Variable | Purpose |
|---|---|
| `GCP_PROJECT_ID` | Target project ID (Go workloads, scripts) |
| `GCP_REGION` | Region, e.g. `us-central1` |
| `GCP_ZONE` | Zone, e.g. `us-central1-a` |
| `TF_VAR_project_id` | Same as `GCP_PROJECT_ID`, for Terraform |
| `TF_VAR_region` | Same as `GCP_REGION` |
| `TF_VAR_zone` | Same as `GCP_ZONE` |
| `TF_VAR_github_repo` | GitHub repo (`owner/name`) for CI/CD trust; project 16 only |
| `GOOGLE_APPLICATION_CREDENTIALS` | Optional path to a credentials file (alternative to ADC) |

## Setup
```sh
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"
export GCP_ZONE="us-central1-a"
export TF_VAR_project_id="$GCP_PROJECT_ID"
export TF_VAR_region="$GCP_REGION"
export TF_VAR_zone="$GCP_ZONE"
gcloud auth application-default login
```
Or put the exports in a local `.env` (git-ignored) and `source .env`.

## Auth
Use `gcloud auth application-default login`, or set `GOOGLE_APPLICATION_CREDENTIALS` to a key file kept outside the repo. Never commit credentials, `.env`, `*.tfvars` or state files.
