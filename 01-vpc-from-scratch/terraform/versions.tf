terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.10"
    }
  }
}

# Auth via Application Default Credentials; project/region come from TF_VAR_*.
provider "google" {
  project = var.project_id
  region  = var.region
}
