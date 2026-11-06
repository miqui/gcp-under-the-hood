output "artifact_registry_repo" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

output "wif_provider" {
  description = "Value for google-github-actions/auth workload_identity_provider."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "deployer_service_account" {
  value = google_service_account.deployer.email
}

output "attestor_name" {
  value = google_binary_authorization_attestor.built.name
}

output "attestor_kms_key_version" {
  value = data.google_kms_crypto_key_version.attestor.name
}

output "cloud_run_url" {
  value = google_cloud_run_v2_service.app.uri
}

output "delivery_pipeline" {
  value = google_clouddeploy_delivery_pipeline.pipeline.name
}
