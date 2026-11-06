output "service_url" {
  value       = google_cloud_run_v2_service.svc.uri
  description = "Cloud Run URL (authenticated; use an identity token)."
}

output "slo_name" {
  value       = google_monitoring_slo.availability.name
  description = "Full SLO resource name used by burn-rate alerts."
}

output "errors_bucket" {
  value       = google_logging_project_bucket_config.errors.id
  description = "Custom log bucket receiving ERROR+ logs."
}

output "sink_writer_identity" {
  value       = google_logging_project_sink.errors_to_bucket.writer_identity
  description = "Service account the sink writes as."
}

output "dashboard_id" {
  value = google_monitoring_dashboard.main.id
}
