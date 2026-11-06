terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.10"
    }
  }
}

# Credentials come from Application Default Credentials; project/region from TF_VAR_*.
provider "google" {
  project = var.project_id
  region  = var.region
}
