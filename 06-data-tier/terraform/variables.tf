# Set via env: TF_VAR_project_id, TF_VAR_region.
variable "project_id" {
  type        = string
  description = "GCP project ID (TF_VAR_project_id)."
}

variable "region" {
  type        = string
  description = "GCP region (TF_VAR_region)."
}

variable "name_prefix" {
  type    = string
  default = "dt06"
}

variable "db_tier" {
  type        = string
  default     = "db-custom-1-3840"
  description = "Smallest tier valid for REGIONAL (HA) Postgres Enterprise."
}

variable "db_deletion_protection" {
  type        = bool
  default     = true
  description = "Cloud SQL deletion protection (API-level). teardown.sh sets it false first."
}

variable "ha_enabled" {
  type    = bool
  default = true
}

variable "retention_days" {
  type        = number
  default     = 0
  description = "If >0, set an unlocked bucket retention policy of this many days."
}
