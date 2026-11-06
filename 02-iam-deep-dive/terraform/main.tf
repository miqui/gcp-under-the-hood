resource "random_id" "suffix" {
  byte_length = 3
}

locals {
  sfx = random_id.suffix.hex
}

resource "google_project_service" "apis" {
  for_each = toset([
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "orgpolicy.googleapis.com",
    "cloudresourcemanager.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false
}

# --- Identities -------------------------------------------------------------
# "caller" is the principal you run as; "target" is the identity it impersonates.
resource "google_service_account" "caller" {
  account_id   = "iamdd-caller-${local.sfx}"
  display_name = "IAM deep dive: caller"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "target" {
  account_id   = "iamdd-target-${local.sfx}"
  display_name = "IAM deep dive: impersonation target"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "wif" {
  account_id   = "iamdd-wif-${local.sfx}"
  display_name = "IAM deep dive: federated workload identity"
  depends_on   = [google_project_service.apis]
}

# --- Resource to protect ----------------------------------------------------
resource "google_storage_bucket" "data" {
  name                        = "${var.project_id}-iamdd-${local.sfx}"
  location                    = var.region
  force_destroy               = true
  uniform_bucket_level_access = true
}

resource "google_storage_bucket_object" "hello" {
  bucket  = google_storage_bucket.data.name
  name    = "hello.txt"
  content = "hello from the protected bucket\n"
}

# --- Additive allow policy at two levels of the hierarchy --------------------
# Project level: effective on EVERY resource below (inheritance is a union).
resource "google_project_iam_member" "target_project_viewer" {
  project = var.project_id
  role    = "roles/storage.objectViewer"
  member  = "serviceAccount:${google_service_account.target.email}"

  # IAM Condition: only grants access to objects in our bucket, and expires.
  condition {
    title       = "only-iamdd-bucket-until-expiry"
    description = "Resource-name prefix + time bound"
    expression  = "resource.name.startsWith(\"projects/_/buckets/${google_storage_bucket.data.name}\") && request.time < timestamp(\"2099-01-01T00:00:00Z\")"
  }
}

# Resource level: a second, additive grant (does not replace the project one).
resource "google_storage_bucket_iam_member" "wif_reader" {
  bucket = google_storage_bucket.data.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.wif.email}"
}

# Caller can also delete (so the deny policy has something to override).
resource "google_storage_bucket_iam_member" "caller_admin" {
  bucket = google_storage_bucket.data.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.caller.email}"
}

# --- Impersonation: caller may mint tokens for target ------------------------
resource "google_service_account_iam_member" "caller_token_creator" {
  service_account_id = google_service_account.target.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.caller.email}"
}

# Whoever runs the tool (your user) may mint tokens for caller SA too, via
# the same role; add manually in the lab (see README) to avoid hardcoding emails.

# --- Deny policy: evaluated BEFORE allow; cannot be overridden by grants ------
resource "google_iam_deny_policy" "no_delete" {
  count  = var.enable_deny_policy ? 1 : 0
  parent = urlencode("cloudresourcemanager.googleapis.com/projects/${var.project_id}")
  name   = "iamdd-no-delete-${local.sfx}"

  display_name = "Deny object delete to caller SA"
  rules {
    deny_rule {
      denied_principals  = ["principal://iam.googleapis.com/projects/-/serviceAccounts/${google_service_account.caller.email}"]
      denied_permissions = ["storage.googleapis.com/objects.delete"]
    }
  }
  depends_on = [google_project_service.apis]
}

# --- Org policy constraint (project-scoped) ----------------------------------
resource "google_org_policy_policy" "no_sa_keys" {
  count  = var.enable_org_policy ? 1 : 0
  name   = "projects/${var.project_id}/policies/iam.disableServiceAccountKeyCreation"
  parent = "projects/${var.project_id}"

  spec {
    rules {
      enforce = "TRUE"
    }
  }
  depends_on = [google_project_service.apis]
}

# --- Workload Identity Federation --------------------------------------------
resource "google_iam_workload_identity_pool" "pool" {
  workload_identity_pool_id = "iamdd-pool-${local.sfx}"
  display_name              = "IAM deep dive pool"
  depends_on                = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "oidc" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "oidc"
  display_name                       = "External OIDC"

  attribute_mapping = {
    "google.subject" = "assertion.sub"
  }
  # Without a condition/mapping, any token from the issuer would be accepted.
  attribute_condition = "assertion.sub == \"${var.oidc_subject}\""

  oidc {
    issuer_uri = var.oidc_issuer_uri
  }
}

# Direct binding: external subject -> impersonate the WIF SA.
resource "google_service_account_iam_member" "wif_user" {
  service_account_id = google_service_account.wif.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.pool.name}/subject/${var.oidc_subject}"
}
