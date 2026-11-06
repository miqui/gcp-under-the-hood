output "lb_ip" {
  description = "Global external Application LB IP (HTTP :80)."
  value       = google_compute_global_address.vip.address
}

output "nlb_ip" {
  description = "Regional passthrough Network LB IP (TCP :8080), or null if disabled."
  value       = var.enable_network_lb ? google_compute_address.nlb_ip[0].address : null
}

output "image" {
  description = "Container image the MIG runs."
  value       = local.image
}

output "registry_repo" {
  description = "Artifact Registry repo path to push to."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.repo.repository_id}"
}

output "mig_name" {
  value = google_compute_region_instance_group_manager.mig.name
}

output "backend_service" {
  value = google_compute_backend_service.be.name
}
