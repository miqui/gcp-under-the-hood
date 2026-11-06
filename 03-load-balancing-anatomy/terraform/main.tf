locals {
  app_port = 8080
  image    = var.container_image != "" ? var.container_image : "${var.region}-docker.pkg.dev/${var.project_id}/${var.name}-repo/lbdemo:latest"
  registry = "${var.region}-docker.pkg.dev"
  tag      = "${var.name}-backend"
}

resource "google_project_service" "apis" {
  for_each           = toset(["compute.googleapis.com", "artifactregistry.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}

# ---------- Artifact Registry + service account ----------
resource "google_artifact_registry_repository" "repo" {
  location      = var.region
  repository_id = "${var.name}-repo"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

resource "google_service_account" "vm" {
  account_id   = "${var.name}-vm"
  display_name = "LB anatomy backend VMs"
}

resource "google_artifact_registry_repository_iam_member" "vm_reader" {
  location   = google_artifact_registry_repository.repo.location
  repository = google_artifact_registry_repository.repo.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.vm.email}"
}

# ---------- Network: custom VPC, subnet, Cloud NAT ----------
resource "google_compute_network" "vpc" {
  name                    = "${var.name}-vpc"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "subnet" {
  name                     = "${var.name}-subnet"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = "10.10.0.0/24"
  private_ip_google_access = true
}

resource "google_compute_router" "router" {
  name    = "${var.name}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# ---------- Firewall ----------
# Health-check probers and Google Front Ends (proxy traffic) share these ranges.
resource "google_compute_firewall" "lb_and_probes" {
  name      = "${var.name}-allow-gfe-hc"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  allow {
    protocol = "tcp"
    ports    = [tostring(local.app_port)]
  }
  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = [local.tag]
}

# Passthrough NLB keeps the client source IP, so clients (and extra probe ranges) must be allowed directly.
resource "google_compute_firewall" "nlb_clients" {
  count     = var.enable_network_lb ? 1 : 0
  name      = "${var.name}-allow-nlb"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  allow {
    protocol = "tcp"
    ports    = [tostring(local.app_port)]
  }
  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.tag]
}

resource "google_compute_firewall" "iap_ssh" {
  name      = "${var.name}-allow-iap-ssh"
  network   = google_compute_network.vpc.id
  direction = "INGRESS"
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["35.235.240.0/20"]
  target_tags   = [local.tag]
}

# ---------- Backends: COS + container via MIG ----------
data "google_compute_image" "cos" {
  family  = "cos-stable"
  project = "cos-cloud"
}

resource "google_compute_instance_template" "tpl" {
  name_prefix  = "${var.name}-tpl-"
  machine_type = var.machine_type
  tags         = [local.tag]

  disk {
    source_image = data.google_compute_image.cos.self_link
    auto_delete  = true
    boot         = true
  }

  network_interface {
    subnetwork = google_compute_subnetwork.subnet.id
    # No access_config: no external IP; egress via Cloud NAT.
  }

  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    user-data = <<-EOT
      #cloud-config
      write_files:
      - path: /etc/systemd/system/lbdemo.service
        permissions: "0644"
        content: |
          [Unit]
          Description=lbdemo container
          After=gcr-online.target docker.socket
          Wants=gcr-online.target
          [Service]
          Environment="HOME=/home/lbdemo"
          ExecStartPre=/usr/bin/docker-credential-gcr configure-docker --registries=${local.registry}
          ExecStart=/usr/bin/docker run --rm --name=lbdemo -p ${local.app_port}:8080 ${local.image}
          ExecStop=/usr/bin/docker stop lbdemo
          Restart=always
          RestartSec=5
      runcmd:
      - systemctl daemon-reload
      - systemctl start lbdemo.service
    EOT
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [google_artifact_registry_repository_iam_member.vm_reader, google_compute_router_nat.nat]
}

resource "google_compute_health_check" "hc" {
  name                = "${var.name}-hc"
  check_interval_sec  = 5
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 2
  http_health_check {
    port         = local.app_port
    request_path = "/healthz"
  }
}

resource "google_compute_region_instance_group_manager" "mig" {
  name               = "${var.name}-mig"
  region             = var.region
  base_instance_name = "${var.name}-vm"
  target_size        = var.instance_count

  version {
    instance_template = google_compute_instance_template.tpl.self_link
  }

  named_port {
    name = "http"
    port = local.app_port
  }

  auto_healing_policies {
    health_check      = google_compute_health_check.hc.id
    initial_delay_sec = 120
  }

  update_policy {
    type                  = "PROACTIVE"
    minimal_action        = "REPLACE"
    max_surge_fixed       = 3
    max_unavailable_fixed = 0
  }
}

# ---------- Global external Application LB ----------
resource "google_compute_global_address" "vip" {
  name       = "${var.name}-vip"
  depends_on = [google_project_service.apis]
}

resource "google_compute_backend_service" "be" {
  name                            = "${var.name}-be"
  protocol                        = "HTTP"
  port_name                       = "http"
  load_balancing_scheme           = "EXTERNAL_MANAGED"
  timeout_sec                     = 30
  connection_draining_timeout_sec = var.draining_timeout_sec
  health_checks                   = [google_compute_health_check.hc.id]

  backend {
    group           = google_compute_region_instance_group_manager.mig.instance_group
    balancing_mode  = var.balancing_mode
    capacity_scaler = var.capacity_scaler
    # Only the attribute matching the balancing mode may be set.
    max_rate_per_instance = var.balancing_mode == "RATE" ? var.max_rate_per_instance : null
    max_utilization       = var.balancing_mode == "UTILIZATION" ? var.max_utilization : null
  }

  log_config {
    enable      = true
    sample_rate = 1.0
  }
}

resource "google_compute_url_map" "um" {
  name            = "${var.name}-urlmap"
  default_service = google_compute_backend_service.be.id
}

resource "google_compute_target_http_proxy" "proxy" {
  name    = "${var.name}-proxy"
  url_map = google_compute_url_map.um.id
}

resource "google_compute_global_forwarding_rule" "fr" {
  name                  = "${var.name}-fr"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.vip.id
  ip_protocol           = "TCP"
  port_range            = "80"
  target                = google_compute_target_http_proxy.proxy.id
}

# ---------- Optional: regional external passthrough Network LB (L4) ----------
resource "google_compute_region_health_check" "nlb_hc" {
  count               = var.enable_network_lb ? 1 : 0
  name                = "${var.name}-nlb-hc"
  region              = var.region
  check_interval_sec  = 5
  timeout_sec         = 5
  healthy_threshold   = 2
  unhealthy_threshold = 2
  http_health_check {
    port         = local.app_port
    request_path = "/healthz"
  }
}

resource "google_compute_region_backend_service" "nlb_be" {
  count                           = var.enable_network_lb ? 1 : 0
  name                            = "${var.name}-nlb-be"
  region                          = var.region
  protocol                        = "TCP"
  load_balancing_scheme           = "EXTERNAL"
  health_checks                   = [google_compute_region_health_check.nlb_hc[0].id]
  connection_draining_timeout_sec = var.draining_timeout_sec

  backend {
    group          = google_compute_region_instance_group_manager.mig.instance_group
    balancing_mode = "CONNECTION"
  }
}

resource "google_compute_address" "nlb_ip" {
  count  = var.enable_network_lb ? 1 : 0
  name   = "${var.name}-nlb-ip"
  region = var.region
}

resource "google_compute_forwarding_rule" "nlb_fr" {
  count                 = var.enable_network_lb ? 1 : 0
  name                  = "${var.name}-nlb-fr"
  region                = var.region
  load_balancing_scheme = "EXTERNAL"
  ip_protocol           = "TCP"
  ip_address            = google_compute_address.nlb_ip[0].address
  port_range            = tostring(local.app_port)
  backend_service       = google_compute_region_backend_service.nlb_be[0].id
}
