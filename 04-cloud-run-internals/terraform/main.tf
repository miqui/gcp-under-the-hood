locals {
  # Public hello image lets `apply` succeed before you push your own image.
  # NOTE: the hello image does NOT serve /info or /work; push the Go image for the experiments.
  image = var.image != "" ? var.image : "us-docker.pkg.dev/cloudrun/container/hello"
}

resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "compute.googleapis.com",
    "artifactregistry.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "repo" {
  location      = var.region
  repository_id = "cloudrun-internals"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

# --- Network for Direct VPC egress (no Serverless VPC Access connector VMs) ---
resource "google_compute_network" "vpc" {
  name                    = "cloudrun-internals"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "egress" {
  name          = "cloudrun-egress"
  region        = var.region
  network       = google_compute_network.vpc.id
  ip_cidr_range = "10.20.0.0/24" # each instance consumes one IP from here
}

resource "google_service_account" "run" {
  account_id   = "cloudrun-internals"
  display_name = "Cloud Run internals demo (least privilege, no roles)"
}

resource "google_cloud_run_v2_service" "svc" {
  name                = "internals"
  location            = var.region
  ingress             = var.ingress
  deletion_protection = false

  template {
    service_account                  = google_service_account.run.email
    max_instance_request_concurrency = var.concurrency
    timeout                          = "60s"

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.vpc.id
        subnetwork = google_compute_subnetwork.egress.id
      }
    }

    containers {
      image = local.image

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = !var.cpu_always_allocated # true => request-based billing
        startup_cpu_boost = var.startup_cpu_boost
      }

      dynamic "env" {
        for_each = var.startup_delay == "" ? [] : [var.startup_delay]
        content {
          name  = "STARTUP_DELAY"
          value = env.value
        }
      }

      startup_probe {
        tcp_socket {
          port = 8080
        }
        period_seconds    = 2
        timeout_seconds   = 1
        failure_threshold = 30
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# Behind the LB the caller is the LB, not a Google identity, so the service must allow allUsers.
# Network-level protection is the ingress setting, not IAM.
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.svc.name
  location = google_cloud_run_v2_service.svc.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# --- Global external HTTP LB -> serverless NEG -> Cloud Run ---
resource "google_compute_region_network_endpoint_group" "neg" {
  name                  = "cloudrun-internals-neg"
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.svc.name
  }
}

resource "google_compute_backend_service" "backend" {
  name                  = "cloudrun-internals-backend"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"

  backend {
    group = google_compute_region_network_endpoint_group.neg.id
  }
}

resource "google_compute_url_map" "urlmap" {
  name            = "cloudrun-internals-urlmap"
  default_service = google_compute_backend_service.backend.id
}

resource "google_compute_target_http_proxy" "proxy" {
  name    = "cloudrun-internals-proxy"
  url_map = google_compute_url_map.urlmap.id
}

resource "google_compute_global_address" "lb_ip" {
  name       = "cloudrun-internals-ip"
  depends_on = [google_project_service.apis]
}

resource "google_compute_global_forwarding_rule" "http" {
  name                  = "cloudrun-internals-http"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.lb_ip.id
  port_range            = "80"
  target                = google_compute_target_http_proxy.proxy.id
}
