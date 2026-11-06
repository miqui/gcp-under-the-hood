variable "project_id" {
  type        = string
  description = "GCP project ID (set via TF_VAR_project_id)."
}

variable "region" {
  type        = string
  description = "GCP region (set via TF_VAR_region)."
}

variable "image" {
  type        = string
  description = "Container image for the service. Default is Google's hello image (always healthy, emits no fault-injectable logs); push the Go image and override for the full lab."
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "fault_error_rate" {
  type        = number
  description = "Initial FAULT_ERROR_RATE (0..1) for the Go service."
  default     = 0
}

variable "fault_latency_ms" {
  type        = number
  description = "Initial FAULT_LATENCY_MS for the Go service."
  default     = 0
}

variable "slo_goal" {
  type        = number
  description = "Availability SLO target (0..1)."
  default     = 0.99
}

variable "slo_rolling_days" {
  type        = number
  description = "SLO compliance window in days."
  default     = 7
}

variable "notification_email" {
  type        = string
  description = "Optional email for alert notifications. Empty = no channel (alerts only appear in the console)."
  default     = ""
}

variable "log_retention_days" {
  type        = number
  description = "Retention for the custom log bucket (min 1)."
  default     = 30
}
