resource "google_service_account" "app" {
  account_id   = "${var.name_prefix}-app"
  display_name = "06-data-tier workload"
  depends_on   = [google_project_service.apis]
}

# Lets the connector call sqladmin to fetch an ephemeral client cert + connect.
resource "google_project_iam_member" "sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# Lets the SA log in as an IAM database user (OAuth token used as password).
resource "google_project_iam_member" "sql_instance_user" {
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# V4 signing without a key file: SA calls iamcredentials.signBlob on itself.
resource "google_service_account_iam_member" "self_sign" {
  service_account_id = google_service_account.app.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.app.email}"
}

# Optional: let YOUR user impersonate the SA (run the tools locally with
# GOOGLE_IMPERSONATE_SERVICE_ACCOUNT / ADC impersonation). Set TF_VAR_impersonator_member.
variable "impersonator_member" {
  type        = string
  default     = ""
  description = "e.g. user:you@example.com; granted tokenCreator on the app SA."
}

resource "google_service_account_iam_member" "impersonator" {
  count              = var.impersonator_member == "" ? 0 : 1
  service_account_id = google_service_account.app.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = var.impersonator_member
}

resource "google_storage_bucket_iam_member" "app_rw" {
  bucket = google_storage_bucket.data.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.app.email}"
}
