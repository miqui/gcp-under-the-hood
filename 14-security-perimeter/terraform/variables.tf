variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id; mirrors GCP_PROJECT_ID)."
  type        = string
}

variable "region" {
  description = "GCP region (set via TF_VAR_region; mirrors GCP_REGION)."
  type        = string
  default     = "us-central1"
}

variable "name_prefix" {
  description = "Prefix for resource names."
  type        = string
  default     = "secperim"
}

variable "key_rotation_period" {
  description = "KEK automatic rotation period (e.g. 7776000s = 90 days). Min 86400s."
  type        = string
  default     = "7776000s"
}

variable "key_destroy_scheduled_duration" {
  description = "How long a destroyed key version stays in DESTROY_SCHEDULED. Immutable after key creation. Min 24h."
  type        = string
  default     = "86400s"
}

variable "iap_tunnel_member" {
  description = "Member (e.g. user:you@example.com) granted IAP TCP tunnel access. Empty = skip IAP IAM."
  type        = string
  default     = ""
}

variable "armor_blocked_cidrs" {
  description = "Source CIDRs denied by the Cloud Armor policy (documentation range by default)."
  type        = list(string)
  default     = ["198.51.100.0/24"]
}

# ---- VPC Service Controls (requires an organization) ----

variable "enable_vpc_sc" {
  description = "Create VPC-SC access level + perimeter. REQUIRES a GCP organization and an existing access policy."
  type        = bool
  default     = false
}

variable "access_policy_id" {
  description = "Numeric ID of the org's Access Context Manager policy (only used if enable_vpc_sc)."
  type        = string
  default     = ""
}

variable "vpc_sc_enforced" {
  description = "false = dry-run (violations logged, not blocked); true = enforced (violations blocked)."
  type        = bool
  default     = false
}

variable "vpc_sc_allowed_members" {
  description = "Members (user:/serviceAccount:) allowed by the access level from anywhere."
  type        = list(string)
  default     = []
}
