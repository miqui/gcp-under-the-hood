locals {
  services = [
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "run.googleapis.com",
    "clouddeploy.googleapis.com",
    "binaryauthorization.googleapis.com",
    "containeranalysis.googleapis.com",
    "cloudkms.googleapis.com",
  ]
  wif_principal_set = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repo}"
}

resource "google_project_service" "apis" {
  for_each           = toset(local.services)
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ---------- Artifact Registry ----------
resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = "supply-chain"
  format        = "DOCKER"
  description   = "Images for the supply-chain lab"
  depends_on    = [google_project_service.apis]
}

# ---------- Service accounts ----------
resource "google_service_account" "deployer" {
  account_id   = "gh-deployer"
  display_name = "GitHub Actions deployer (via WIF)"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "runtime" {
  account_id   = "run-runtime"
  display_name = "Cloud Run runtime identity"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "deploy_exec" {
  account_id   = "clouddeploy-exec"
  display_name = "Cloud Deploy execution identity"
  depends_on   = [google_project_service.apis]
}

# ---------- Workload Identity Federation (keyless GitHub -> GCP) ----------
resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Actions"
  depends_on                = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-oidc"
  display_name                       = "GitHub OIDC"

  attribute_mapping = {
    "google.subject"             = "assertion.sub"
    "attribute.repository"       = "assertion.repository"
    "attribute.ref"              = "assertion.ref"
    "attribute.repository_owner" = "assertion.repository_owner"
  }

  # Tokens from any other repo are rejected at STS, before IAM is consulted.
  attribute_condition = "assertion.repository == \"${var.github_repo}\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "wif_impersonate" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = local.wif_principal_set
}

resource "google_artifact_registry_repository_iam_member" "deployer_push" {
  repository = google_artifact_registry_repository.images.name
  location   = var.region
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_project_iam_member" "deployer_roles" {
  for_each = toset([
    "roles/cloudbuild.builds.editor",
    "roles/clouddeploy.releaser",
    "roles/binaryauthorization.attestorsViewer",
    "roles/cloudkms.signerVerifier",
    "roles/containeranalysis.notes.attacher",
    "roles/containeranalysis.occurrences.editor",
  ])
  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_service_account_iam_member" "deployer_act_as_exec" {
  service_account_id = google_service_account.deploy_exec.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}

# ---------- Binary Authorization ----------
resource "google_kms_key_ring" "attestor" {
  name       = "attestor-ring"
  location   = var.region
  depends_on = [google_project_service.apis]
}

resource "google_kms_crypto_key" "attestor" {
  name     = "attestor-key"
  key_ring = google_kms_key_ring.attestor.id
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm = "EC_SIGN_P256_SHA256"
  }
}

data "google_kms_crypto_key_version" "attestor" {
  crypto_key = google_kms_crypto_key.attestor.id
}

resource "google_container_analysis_note" "attestor" {
  name = "build-attestor-note"

  attestation_authority {
    hint {
      human_readable_name = "Built by trusted CI"
    }
  }
  depends_on = [google_project_service.apis]
}

resource "google_binary_authorization_attestor" "built" {
  name = "built-by-ci"

  attestation_authority_note {
    note_reference = google_container_analysis_note.attestor.name

    public_keys {
      id = data.google_kms_crypto_key_version.attestor.id
      pkix_public_key {
        public_key_pem      = data.google_kms_crypto_key_version.attestor.public_key[0].pem
        signature_algorithm = "ECDSA_P256_SHA256"
      }
    }
  }
}

resource "google_binary_authorization_policy" "policy" {
  # The Google hello image used for bootstrapping has no attestation.
  admission_whitelist_patterns {
    name_pattern = "us-docker.pkg.dev/cloudrun/container/*"
  }

  default_admission_rule {
    evaluation_mode         = "REQUIRE_ATTESTATION"
    enforcement_mode        = var.binauthz_enforcement_mode
    require_attestations_by = [google_binary_authorization_attestor.built.name]
  }
}

# ---------- Cloud Run service ----------
resource "google_cloud_run_v2_service" "app" {
  name                = var.service_name
  location            = var.region
  deletion_protection = false
  ingress             = "INGRESS_TRAFFIC_ALL"

  binary_authorization {
    use_default = true
  }

  template {
    service_account = google_service_account.runtime.email
    containers {
      image = var.initial_image
    }
  }

  # Cloud Deploy owns revisions and traffic after bootstrap.
  lifecycle {
    ignore_changes = [template, traffic, client, client_version]
  }

  depends_on = [google_binary_authorization_policy.policy, google_project_service.apis]
}

# ---------- Cloud Deploy (canary between Cloud Run revisions) ----------
resource "google_project_iam_member" "deploy_exec_roles" {
  for_each = toset([
    "roles/clouddeploy.jobRunner",
    "roles/run.developer",
    "roles/logging.logWriter",
    "roles/artifactregistry.reader",
  ])
  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.deploy_exec.email}"
}

resource "google_service_account_iam_member" "deploy_exec_act_as_runtime" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deploy_exec.email}"
}

resource "google_clouddeploy_target" "prod" {
  name     = "prod-run"
  location = var.region

  run {
    location = "projects/${var.project_id}/locations/${var.region}"
  }

  execution_configs {
    usages          = ["RENDER", "DEPLOY"]
    service_account = google_service_account.deploy_exec.email
  }

  depends_on = [google_project_service.apis]
}

resource "google_clouddeploy_delivery_pipeline" "pipeline" {
  name     = "supply-chain-pipeline"
  location = var.region

  serial_pipeline {
    stages {
      target_id = google_clouddeploy_target.prod.name

      strategy {
        canary {
          runtime_config {
            cloud_run {
              automatic_traffic_control = true
            }
          }
          canary_deployment {
            percentages = [25, 50]
            verify      = false
          }
        }
      }
    }
  }
}
