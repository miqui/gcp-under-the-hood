variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id / GCP_PROJECT_ID)."
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region / GCP_REGION)."
  type        = string
}

variable "github_repo" {
  description = "GitHub repo allowed to impersonate the deployer SA, as OWNER/REPO (TF_VAR_github_repo)."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repo))
    error_message = "github_repo must look like OWNER/REPO."
  }
}

variable "service_name" {
  description = "Cloud Run service name."
  type        = string
  default     = "supply-chain-demo"
}

variable "initial_image" {
  description = "Bootstrap image for the Cloud Run service (replaced by Cloud Deploy releases)."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "binauthz_enforcement_mode" {
  description = "ENFORCED_BLOCK_AND_AUDIT_LOG blocks unattested images; DRYRUN_AUDIT_LOG_ONLY only logs."
  type        = string
  default     = "DRYRUN_AUDIT_LOG_ONLY"

  validation {
    condition     = contains(["ENFORCED_BLOCK_AND_AUDIT_LOG", "DRYRUN_AUDIT_LOG_ONLY"], var.binauthz_enforcement_mode)
    error_message = "Must be ENFORCED_BLOCK_AND_AUDIT_LOG or DRYRUN_AUDIT_LOG_ONLY."
  }
}
