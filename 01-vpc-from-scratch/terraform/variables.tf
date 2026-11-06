variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id)."
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region)."
  type        = string
}

variable "zone_suffix" {
  description = "Zone letter appended to region."
  type        = string
  default     = "b"
}

variable "machine_type" {
  type    = string
  default = "e2-micro"
}

variable "enable_nat" {
  description = "Create Cloud NAT on VPC A. Toggle to observe egress behaviour."
  type        = bool
  default     = true
}

variable "enable_pga" {
  description = "Private Google Access on the private subnet."
  type        = bool
  default     = true
}

variable "enable_deny_rule" {
  description = "Priority-100 deny rule that beats the priority-1000 allow (firewall priority demo)."
  type        = bool
  default     = false
}

variable "enable_b_c_peering" {
  description = "Peer B<->C. A<->B is always peered; A<->C is NOT, showing non-transitivity."
  type        = bool
  default     = true
}
