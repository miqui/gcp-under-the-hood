data "google_project" "this" {
  project_id = var.project_id
}

resource "google_project_service" "apis" {
  for_each = toset([
    "cloudkms.googleapis.com",
    "secretmanager.googleapis.com",
    "storage.googleapis.com",
    "compute.googleapis.com",
    "iap.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false
}

# ---------------- Cloud KMS: key ring + KEK ----------------
# Key rings and keys can NEVER be deleted; only key versions are destroyed.
resource "google_kms_key_ring" "ring" {
  name       = "${var.name_prefix}-ring"
  location   = var.region
  depends_on = [google_project_service.apis]
}

# Symmetric KEK used to wrap DEKs (envelope encryption) and as CMEK.
resource "google_kms_crypto_key" "kek" {
  name                       = "${var.name_prefix}-kek"
  key_ring                   = google_kms_key_ring.ring.id
  purpose                    = "ENCRYPT_DECRYPT"
  rotation_period            = var.key_rotation_period
  destroy_scheduled_duration = var.key_destroy_scheduled_duration

  version_template {
    algorithm        = "GOOGLE_SYMMETRIC_ENCRYPTION"
    protection_level = "SOFTWARE"
  }
}

# ---------------- Service agents get CMEK access ----------------
data "google_storage_project_service_account" "gcs" {
  project    = var.project_id
  depends_on = [google_project_service.apis]
}

resource "google_kms_crypto_key_iam_member" "gcs_agent" {
  crypto_key_id = google_kms_crypto_key.kek.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${data.google_storage_project_service_account.gcs.email_address}"
}

# The Secret Manager service agent only exists once the API has been used;
# its address is deterministic, so we reference it by name.
resource "google_kms_crypto_key_iam_member" "sm_agent" {
  crypto_key_id = google_kms_crypto_key.kek.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:service-${data.google_project.this.number}@gcp-sa-secretmanager.iam.gserviceaccount.com"
}

# ---------------- Secret Manager (CMEK, versions) ----------------
resource "google_secret_manager_secret" "demo" {
  secret_id = "${var.name_prefix}-demo"

  replication {
    user_managed {
      replicas {
        location = var.region
        customer_managed_encryption {
          kms_key_name = google_kms_crypto_key.kek.id
        }
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_kms_crypto_key_iam_member.sm_agent,
  ]
}

# Placeholder value only; real secrets must not live in Terraform state.
resource "google_secret_manager_secret_version" "v1" {
  secret      = google_secret_manager_secret.demo.id
  secret_data = "demo-value-not-a-real-secret"
}

# ---------------- CMEK bucket (the thing a perimeter protects) ----------------
resource "google_storage_bucket" "cmek" {
  name                        = "${var.project_id}-${var.name_prefix}-cmek"
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = true

  encryption {
    default_kms_key_name = google_kms_crypto_key.kek.id
  }

  depends_on = [google_kms_crypto_key_iam_member.gcs_agent]
}

# ---------------- Cloud Armor (standalone edge policy) ----------------
resource "google_compute_security_policy" "armor" {
  name        = "${var.name_prefix}-armor"
  description = "Deny listed CIDRs, preconfigured XSS rule, throttle, default allow"

  rule {
    action      = "deny(403)"
    priority    = 1000
    description = "Block listed source ranges"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = var.armor_blocked_cidrs
      }
    }
  }

  rule {
    action      = "deny(403)"
    priority    = 2000
    description = "OWASP XSS preconfigured rule"
    match {
      expr {
        expression = "evaluatePreconfiguredExpr('xss-v33-stable')"
      }
    }
  }

  rule {
    action      = "throttle"
    priority    = 3000
    description = "Per-IP rate limit"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      enforce_on_key = "IP"
      rate_limit_threshold {
        count        = 100
        interval_sec = 60
      }
    }
  }

  rule {
    action      = "allow"
    priority    = 2147483647
    description = "Default allow"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# ---------------- IAP TCP forwarding (no external IPs needed) ----------------
resource "google_compute_network" "iap" {
  name                    = "${var.name_prefix}-net"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_firewall" "iap_ssh" {
  name      = "${var.name_prefix}-allow-iap-ssh"
  network   = google_compute_network.iap.name
  direction = "INGRESS"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  # Google's IAP TCP forwarding source range.
  source_ranges = ["35.235.240.0/20"]
}

resource "google_project_iam_member" "iap_tunnel" {
  count   = var.iap_tunnel_member == "" ? 0 : 1
  project = var.project_id
  role    = "roles/iap.tunnelResourceAccessor"
  member  = var.iap_tunnel_member
}

# ---------------- VPC Service Controls (org required, toggled) ----------------
resource "google_access_context_manager_access_level" "trusted" {
  count  = var.enable_vpc_sc ? 1 : 0
  parent = "accessPolicies/${var.access_policy_id}"
  name   = "accessPolicies/${var.access_policy_id}/accessLevels/${replace(var.name_prefix, "-", "_")}_trusted"
  title  = "${var.name_prefix} trusted identities"

  basic {
    conditions {
      members = var.vpc_sc_allowed_members
    }
  }
}

resource "google_access_context_manager_service_perimeter" "perimeter" {
  count  = var.enable_vpc_sc ? 1 : 0
  parent = "accessPolicies/${var.access_policy_id}"
  name   = "accessPolicies/${var.access_policy_id}/servicePerimeters/${replace(var.name_prefix, "-", "_")}_perimeter"
  title  = "${var.name_prefix}_perimeter"

  # Dry-run uses the separate `spec` block; enforced uses `status`.
  use_explicit_dry_run_spec = !var.vpc_sc_enforced

  dynamic "status" {
    for_each = var.vpc_sc_enforced ? [1] : []
    content {
      resources           = ["projects/${data.google_project.this.number}"]
      restricted_services = ["storage.googleapis.com", "secretmanager.googleapis.com", "cloudkms.googleapis.com"]
      access_levels       = [google_access_context_manager_access_level.trusted[0].name]
    }
  }

  dynamic "spec" {
    for_each = var.vpc_sc_enforced ? [] : [1]
    content {
      resources           = ["projects/${data.google_project.this.number}"]
      restricted_services = ["storage.googleapis.com", "secretmanager.googleapis.com", "cloudkms.googleapis.com"]
      access_levels       = [google_access_context_manager_access_level.trusted[0].name]
    }
  }
}
