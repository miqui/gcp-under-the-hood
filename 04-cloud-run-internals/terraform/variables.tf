variable "project_id" {
  type        = string
  description = "GCP project ID. Set via TF_VAR_project_id (from GCP_PROJECT_ID)."
}

variable "region" {
  type        = string
  description = "GCP region. Set via TF_VAR_region (from GCP_REGION)."
}

variable "image" {
  type        = string
  description = "Container image. Leave empty to use Google's public hello image; set to your Artifact Registry image after pushing."
  default     = ""
}

variable "min_instances" {
  type        = number
  description = "Minimum instances kept warm (billed at idle rate). 0 = scale to zero."
  default     = 0
}

variable "max_instances" {
  type        = number
  description = "Maximum instances (hard cap on scale-out)."
  default     = 5
}

variable "concurrency" {
  type        = number
  description = "Max concurrent requests per instance (1-1000)."
  default     = 10
}

variable "cpu_always_allocated" {
  type        = bool
  description = "true = instance-based billing (CPU always on); false = request-based (CPU throttled outside requests)."
  default     = false
}

variable "startup_cpu_boost" {
  type        = bool
  description = "Temporarily allocate extra CPU during container startup."
  default     = true
}

variable "ingress" {
  type        = string
  description = "Ingress setting: INGRESS_TRAFFIC_ALL, INGRESS_TRAFFIC_INTERNAL_ONLY, or INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER."
  default     = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  validation {
    condition     = contains(["INGRESS_TRAFFIC_ALL", "INGRESS_TRAFFIC_INTERNAL_ONLY", "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"], var.ingress)
    error_message = "Invalid ingress value."
  }
}

variable "startup_delay" {
  type        = string
  description = "Artificial startup delay passed as STARTUP_DELAY to the app (e.g. \"3s\"); empty for none."
  default     = ""
}
