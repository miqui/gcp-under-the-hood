variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id)."
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region)."
  type        = string
}

variable "cloud_routing_mode" {
  description = "Dynamic routing mode of the 'cloud' VPC: GLOBAL or REGIONAL. Experiment with this."
  type        = string
  default     = "REGIONAL"
  validation {
    condition     = contains(["GLOBAL", "REGIONAL"], var.cloud_routing_mode)
    error_message = "Must be GLOBAL or REGIONAL."
  }
}

variable "tunnel1_med" {
  description = "Advertised route priority (MED) on tunnel 0 BGP sessions. Lower wins."
  type        = number
  default     = 100
}

variable "tunnel2_med" {
  description = "Advertised route priority (MED) on tunnel 1 BGP sessions. Lower wins."
  type        = number
  default     = 200
}

variable "cloud_cidr" {
  type    = string
  default = "10.10.0.0/24"
}

variable "onprem_cidr" {
  type    = string
  default = "192.168.10.0/24"
}

variable "spoke_cidr" {
  type    = string
  default = "10.20.0.0/24"
}

variable "enable_vms" {
  description = "Create two small test VMs (one per VPC). Costs a little."
  type        = bool
  default     = true
}
