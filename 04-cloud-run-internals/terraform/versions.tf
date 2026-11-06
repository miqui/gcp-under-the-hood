terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

# Auth comes from Application Default Credentials (gcloud auth application-default login).
# No credentials or IDs are hardcoded; project/region come from TF_VAR_project_id / TF_VAR_region.
provider "google" {
  project = var.project_id
  region  = var.region
}
