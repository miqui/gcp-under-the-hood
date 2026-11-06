output "project_id" {
  value = var.project_id
}

output "zone" {
  value = local.zone
}

output "internal_ips" {
  value = {
    a_public  = google_compute_instance.a_public.network_interface[0].network_ip
    a_private = google_compute_instance.a_private.network_interface[0].network_ip
    b         = google_compute_instance.b.network_interface[0].network_ip
    c         = google_compute_instance.c.network_interface[0].network_ip
  }
}

output "ssh_a_private" {
  value = "gcloud compute ssh a-private-vm --zone ${local.zone} --tunnel-through-iap"
}

output "bucket" {
  value = google_storage_bucket.bin.name
}
