variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id)"
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region)"
  type        = string
}

variable "zone_suffix" {
  description = "Zone letter within the region; the cluster is zonal (single control plane, cheaper)"
  type        = string
  default     = "b"
}

variable "authorized_cidr" {
  description = "CIDR allowed to reach the public control-plane endpoint (e.g. your IP/32). Empty = nobody external."
  type        = string
  default     = ""
}

variable "node_count" {
  type    = number
  default = 1
}

variable "machine_type" {
  type    = string
  default = "e2-medium"
}

variable "create_autopilot" {
  description = "Also create an Autopilot cluster for the Autopilot-vs-Standard comparison (extra cost)"
  type        = bool
  default     = false
}
