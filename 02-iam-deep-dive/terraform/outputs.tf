output "caller_sa_email" { value = google_service_account.caller.email }
output "target_sa_email" { value = google_service_account.target.email }
output "wif_sa_email" { value = google_service_account.wif.email }
output "bucket_name" { value = google_storage_bucket.data.name }

output "wif_audience" {
  description = "Audience for the STS exchange."
  value       = "//iam.googleapis.com/${google_iam_workload_identity_pool_provider.oidc.name}"
}
