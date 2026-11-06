output "cloud_inbound_dns_ip" {
  value = google_compute_address.cloud_inbound.address
}

output "onprem_inbound_dns_ip" {
  value = google_compute_address.onprem_inbound.address
}

output "cloud_vm_ip" {
  value = cidrhost(var.cloud_cidr, 10)
}

output "onprem_vm_ip" {
  value = cidrhost(var.onprem_cidr, 10)
}

output "zone" {
  value = "${var.region}-a"
}
