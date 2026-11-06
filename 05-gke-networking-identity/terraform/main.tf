locals {
  zone = "${var.region}-${var.zone_suffix}"
}

resource "google_project_service" "apis" {
  for_each = toset([
    "compute.googleapis.com",
    "container.googleapis.com",
    "storage.googleapis.com",
  ])
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# --- Network: VPC-native needs secondary ranges for Pods and Services (alias IPs) ---
resource "google_compute_network" "vpc" {
  name                    = "gke-lab-vpc"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "nodes" {
  name                     = "gke-lab-nodes"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = "10.10.0.0/24" # node IPs
  private_ip_google_access = true

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.20.0.0/20" # each node gets a /24 alias range from here
  }
  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.30.0.0/24" # ClusterIP VIPs
  }
}

# Private nodes have no external IP: Cloud NAT gives them egress (image pulls etc.)
resource "google_compute_router" "router" {
  name    = "gke-lab-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "gke-lab-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# --- Standard private cluster with Dataplane V2 + Workload Identity ---
resource "google_container_cluster" "standard" {
  name     = "gke-lab-standard"
  location = local.zone # zonal: single control plane, cheaper
  project  = var.project_id

  network    = google_compute_network.vpc.id
  subnetwork = google_compute_subnetwork.nodes.id

  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false

  networking_mode = "VPC_NATIVE"
  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Dataplane V2 = eBPF/Cilium; replaces kube-proxy/iptables and enforces NetworkPolicy natively
  datapath_provider = "ADVANCED_DATAPATH"

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28" # control plane VPC is peered into ours
  }

  dynamic "master_authorized_networks_config" {
    for_each = var.authorized_cidr == "" ? [] : [1]
    content {
      cidr_blocks {
        cidr_block   = var.authorized_cidr
        display_name = "operator"
      }
    }
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  depends_on = [google_compute_router_nat.nat]
}

resource "google_container_node_pool" "default" {
  name       = "pool-1"
  cluster    = google_container_cluster.standard.id
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    disk_size_gb = 30
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    workload_metadata_config {
      mode = "GKE_METADATA" # GKE metadata server intercepts 169.254.169.254 per pod
    }
  }
}

# --- Optional Autopilot cluster for comparison ---
resource "google_container_cluster" "autopilot" {
  count               = var.create_autopilot ? 1 : 0
  name                = "gke-lab-autopilot"
  location            = var.region
  project             = var.project_id
  enable_autopilot    = true
  network             = google_compute_network.vpc.id
  subnetwork          = google_compute_subnetwork.nodes.id
  deletion_protection = false

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }
  depends_on = [google_compute_router_nat.nat]
}

# --- Workload Identity Federation for GKE: direct principal:// binding, no GSA ---
resource "google_storage_bucket" "demo" {
  name                        = "${var.project_id}-gke-lab-demo"
  location                    = var.region
  force_destroy               = true
  uniform_bucket_level_access = true
  depends_on                  = [google_project_service.apis]
}

data "google_project" "this" {
  project_id = var.project_id
}

resource "google_storage_bucket_iam_member" "ksa_reader" {
  bucket = google_storage_bucket.demo.name
  role   = "roles/storage.objectViewer"
  # principal://.../subject/ns/<namespace>/sa/<ksa>
  member = "principal://iam.googleapis.com/projects/${data.google_project.this.number}/locations/global/workloadIdentityPools/${var.project_id}.svc.id.goog/subject/ns/demo/sa/gcs-reader"
}
