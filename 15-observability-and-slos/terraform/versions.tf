terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.10"
    }
  }
}

# Project and region come from TF_VAR_project_id / TF_VAR_region.
provider "google" {
  project = var.project_id
  region  = var.region
}
