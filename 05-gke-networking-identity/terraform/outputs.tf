output "cluster_name" {
  value = google_container_cluster.standard.name
}

output "cluster_zone" {
  value = local.zone
}

output "bucket" {
  value = google_storage_bucket.demo.name
}

output "get_credentials" {
  value = "gcloud container clusters get-credentials ${google_container_cluster.standard.name} --zone ${local.zone} --project ${var.project_id}"
}

output "ksa_principal" {
  value = google_storage_bucket_iam_member.ksa_reader.member
}
