locals {
  zone = "${var.region}-${var.zone_suffix}"
}

resource "google_project_service" "apis" {
  for_each           = toset(["compute.googleapis.com", "networkmanagement.googleapis.com", "storage.googleapis.com", "iap.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}

# ---------- VPC A: custom mode, no auto subnets ----------
resource "google_compute_network" "a" {
  name                    = "vpc-a"
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "a_public" {
  name          = "a-public"
  network       = google_compute_network.a.id
  region        = var.region
  ip_cidr_range = "10.10.0.0/24"
}

resource "google_compute_subnetwork" "a_private" {
  name                     = "a-private"
  network                  = google_compute_network.a.id
  region                   = var.region
  ip_cidr_range            = "10.10.1.0/24"
  private_ip_google_access = var.enable_pga
}

resource "google_compute_network" "b" {
  name                    = "vpc-b"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "b" {
  name          = "b-sub"
  network       = google_compute_network.b.id
  region        = var.region
  ip_cidr_range = "10.20.0.0/24"
}

resource "google_compute_network" "c" {
  name                    = "vpc-c"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "c" {
  name          = "c-sub"
  network       = google_compute_network.c.id
  region        = var.region
  ip_cidr_range = "10.30.0.0/24"
}

# ---------- Peering: A<->B, B<->C (A<->C intentionally absent) ----------
resource "google_compute_network_peering" "a_to_b" {
  name         = "a-to-b"
  network      = google_compute_network.a.self_link
  peer_network = google_compute_network.b.self_link
}

resource "google_compute_network_peering" "b_to_a" {
  name         = "b-to-a"
  network      = google_compute_network.b.self_link
  peer_network = google_compute_network.a.self_link
  depends_on   = [google_compute_network_peering.a_to_b]
}

resource "google_compute_network_peering" "b_to_c" {
  count        = var.enable_b_c_peering ? 1 : 0
  name         = "b-to-c"
  network      = google_compute_network.b.self_link
  peer_network = google_compute_network.c.self_link
  depends_on   = [google_compute_network_peering.b_to_a]
}

resource "google_compute_network_peering" "c_to_b" {
  count        = var.enable_b_c_peering ? 1 : 0
  name         = "c-to-b"
  network      = google_compute_network.c.self_link
  peer_network = google_compute_network.b.self_link
  depends_on   = [google_compute_network_peering.b_to_c]
}

# ---------- Routes: two custom routes, same destination, different priority ----------
# Lowest priority number wins. 203.0.113.0/24 is TEST-NET-3 (never routable).
resource "google_compute_route" "demo_primary" {
  name              = "demo-primary"
  network           = google_compute_network.a.name
  dest_range        = "203.0.113.0/24"
  next_hop_instance = google_compute_instance.a_public.self_link
  priority          = 100
  tags              = ["route-demo"]
}

resource "google_compute_route" "demo_backup" {
  name              = "demo-backup"
  network           = google_compute_network.a.name
  dest_range        = "203.0.113.0/24"
  next_hop_instance = google_compute_instance.a_private.self_link
  priority          = 200
  tags              = ["route-demo"]
}

# ---------- Firewall (stateful, enforced at each VM's vNIC) ----------
resource "google_compute_firewall" "iap_ssh" {
  for_each      = { a = google_compute_network.a.name, b = google_compute_network.b.name, c = google_compute_network.c.name }
  name          = "${each.key}-allow-iap-ssh"
  network       = each.value
  direction     = "INGRESS"
  priority      = 1000
  source_ranges = ["35.235.240.0/20"]
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

# Custom VPCs have NO default allow-internal rule: only the implied deny-ingress / allow-egress exist.
resource "google_compute_firewall" "internal" {
  for_each = {
    a = { net = google_compute_network.a.name, ranges = ["10.10.0.0/16", "10.20.0.0/16"] }
    b = { net = google_compute_network.b.name, ranges = ["10.10.0.0/16", "10.20.0.0/16", "10.30.0.0/16"] }
    c = { net = google_compute_network.c.name, ranges = ["10.20.0.0/16", "10.30.0.0/16"] }
  }
  name          = "${each.key}-allow-internal"
  network       = each.value.net
  direction     = "INGRESS"
  priority      = 1000
  source_ranges = each.value.ranges
  allow {
    protocol = "icmp"
  }
  allow {
    protocol = "tcp"
    ports    = ["8080"]
  }
}

resource "google_compute_firewall" "a_deny_8080" {
  count         = var.enable_deny_rule ? 1 : 0
  name          = "a-deny-8080-prio100"
  network       = google_compute_network.a.name
  direction     = "INGRESS"
  priority      = 100
  source_ranges = ["10.0.0.0/8"]
  deny {
    protocol = "tcp"
    ports    = ["8080"]
  }
}

# ---------- Cloud NAT (software-defined, programmed into the host networking stack; not a VM) ----------
resource "google_compute_router" "a" {
  count   = var.enable_nat ? 1 : 0
  name    = "a-router"
  network = google_compute_network.a.id
  region  = var.region
}

resource "google_compute_router_nat" "a" {
  count                              = var.enable_nat ? 1 : 0
  name                               = "a-nat"
  router                             = google_compute_router.a[0].name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    = google_compute_subnetwork.a_private.id
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }
  log_config {
    enable = true
    filter = "ALL"
  }
}

# ---------- Workload binary delivered through GCS (fetched via Private Google Access) ----------
resource "google_storage_bucket" "bin" {
  name                        = "${var.project_id}-vpc01-bin"
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = true
  depends_on                  = [google_project_service.apis]
}

resource "google_storage_bucket_object" "probe" {
  name   = "probe"
  bucket = google_storage_bucket.bin.name
  source = "${path.module}/../go/bin/probe" # built by scripts/build.sh
}

resource "google_service_account" "vm" {
  account_id   = "vpc01-vm"
  display_name = "vpc01 lab VMs"
}

resource "google_storage_bucket_iam_member" "vm_read" {
  bucket = google_storage_bucket.bin.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.vm.email}"
}

locals {
  # Debian 12 GCE images ship gcloud; it reaches storage.googleapis.com via PGA / NAT / external IP.
  startup = <<-S
    #!/bin/bash
    until gcloud storage cp gs://${google_storage_bucket.bin.name}/probe /usr/local/bin/probe; do sleep 5; done
    chmod +x /usr/local/bin/probe
    nohup /usr/local/bin/probe serve -addr :8080 >/var/log/probe.log 2>&1 &
  S
}

# ---------- VMs ----------
resource "google_compute_instance" "a_public" {
  name         = "a-public-vm"
  zone         = local.zone
  machine_type = var.machine_type
  tags         = ["route-demo"]
  boot_disk {
    initialize_params { image = "debian-cloud/debian-12" }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.a_public.id
    access_config {} # ephemeral external IP: 1:1 NAT at the host
  }
  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }
  can_ip_forward          = true
  metadata_startup_script = local.startup
  depends_on              = [google_storage_bucket_object.probe, google_storage_bucket_iam_member.vm_read]
}

resource "google_compute_instance" "a_private" {
  name         = "a-private-vm"
  zone         = local.zone
  machine_type = var.machine_type
  tags         = ["route-demo"]
  boot_disk {
    initialize_params { image = "debian-cloud/debian-12" }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.a_private.id # no external IP
  }
  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }
  can_ip_forward          = true
  metadata_startup_script = local.startup
  depends_on              = [google_storage_bucket_object.probe, google_storage_bucket_iam_member.vm_read]
}

resource "google_compute_instance" "b" {
  name         = "b-vm"
  zone         = local.zone
  machine_type = var.machine_type
  boot_disk {
    initialize_params { image = "debian-cloud/debian-12" }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.b.id
    access_config {}
  }
  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }
  metadata_startup_script = local.startup
  depends_on              = [google_storage_bucket_object.probe, google_storage_bucket_iam_member.vm_read]
}

resource "google_compute_instance" "c" {
  name         = "c-vm"
  zone         = local.zone
  machine_type = var.machine_type
  boot_disk {
    initialize_params { image = "debian-cloud/debian-12" }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.c.id
    access_config {}
  }
  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }
  metadata_startup_script = local.startup
  depends_on              = [google_storage_bucket_object.probe, google_storage_bucket_iam_member.vm_read]
}

# ---------- Connectivity Tests (static config analysis; no packets sent) ----------
resource "google_network_management_connectivity_test" "a_to_b" {
  name = "a-private-to-b"
  source {
    instance = google_compute_instance.a_private.id
  }
  destination {
    instance = google_compute_instance.b.id
    port     = 8080
  }
  protocol   = "TCP"
  depends_on = [google_compute_network_peering.b_to_a]
}

resource "google_network_management_connectivity_test" "a_to_c" {
  name = "a-private-to-c-expect-fail"
  source {
    instance = google_compute_instance.a_private.id
  }
  destination {
    instance = google_compute_instance.c.id
    port     = 8080
  }
  protocol = "TCP"
}
