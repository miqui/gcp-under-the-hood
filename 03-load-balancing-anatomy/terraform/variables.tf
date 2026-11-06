variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id, e.g. from GCP_PROJECT_ID)."
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region, e.g. from GCP_REGION)."
  type        = string
}

variable "name" {
  description = "Prefix for all resource names."
  type        = string
  default     = "lbanat"
}

variable "container_image" {
  description = "Full image URL to run. Empty = <region>-docker.pkg.dev/<project>/<name>-repo/lbdemo:latest (the Artifact Registry repo created here)."
  type        = string
  default     = ""
}

variable "machine_type" {
  description = "Machine type for backends."
  type        = string
  default     = "e2-small"
}

variable "instance_count" {
  description = "Number of instances in the regional MIG (spread across zones)."
  type        = number
  default     = 3
}

variable "balancing_mode" {
  description = "Backend balancing mode for the global LB: RATE or UTILIZATION."
  type        = string
  default     = "RATE"

  validation {
    condition     = contains(["RATE", "UTILIZATION"], var.balancing_mode)
    error_message = "balancing_mode must be RATE or UTILIZATION."
  }
}

variable "max_rate_per_instance" {
  description = "Max requests/sec per instance (RATE mode)."
  type        = number
  default     = 50
}

variable "max_utilization" {
  description = "Target CPU utilization 0.0-1.0 (UTILIZATION mode)."
  type        = number
  default     = 0.6
}

variable "capacity_scaler" {
  description = "Fraction (0-1) of the configured capacity the backend may use; 0 drains it."
  type        = number
  default     = 1.0
}

variable "draining_timeout_sec" {
  description = "Connection draining timeout on the backend services."
  type        = number
  default     = 30
}

variable "enable_network_lb" {
  description = "Also create a regional external passthrough Network LB over the same instances."
  type        = bool
  default     = false
}
