output "kek_id" {
  description = "Full resource name of the KEK (pass to the Go tool as -key)."
  value       = google_kms_crypto_key.kek.id
}

output "secret_id" {
  value = google_secret_manager_secret.demo.secret_id
}

output "bucket_name" {
  value = google_storage_bucket.cmek.name
}

output "armor_policy" {
  value = google_compute_security_policy.armor.name
}

output "vpc_sc_perimeter" {
  description = "Perimeter name, or null when VPC-SC is disabled."
  value       = try(google_access_context_manager_service_perimeter.perimeter[0].name, null)
}
