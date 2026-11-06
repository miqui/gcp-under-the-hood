variable "project_id" {
  description = "GCP project ID (set via TF_VAR_project_id)"
  type        = string
}

variable "region" {
  description = "BigQuery dataset location/region (set via TF_VAR_region). The public bikeshare data is in US, so use US for the sample load."
  type        = string
  default     = "US"
}

variable "dataset_id" {
  description = "Dataset to hold the lab tables"
  type        = string
  default     = "bq_internals_lab"
}
