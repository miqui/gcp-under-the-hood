output "lb_ip" {
  description = "Global LB IP (http://<ip>/info). Can take several minutes to start serving."
  value       = google_compute_global_address.lb_ip.address
}

output "run_url" {
  description = "Direct run.app URL (blocked with 403/404 when ingress is internal-LB-only)."
  value       = google_cloud_run_v2_service.svc.uri
}

output "image_repo" {
  description = "Artifact Registry path to push the Go image to."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.repo.repository_id}"
}

output "service_name" {
  value = google_cloud_run_v2_service.svc.name
}
