variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id)"
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region)"
  type        = string
}

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "psds"
}

variable "push_endpoint" {
  description = "Optional HTTPS endpoint for the push subscription demo. Empty = no push subscription is created."
  type        = string
  default     = ""
}
