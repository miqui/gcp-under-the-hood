resource "google_storage_bucket" "data" {
  name                        = "${var.project_id}-${var.name_prefix}-data-${random_id.suffix.hex}"
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true # ACLs disabled; IAM only
  public_access_prevention    = "enforced"
  force_destroy               = true # lab only

  versioning {
    enabled = true
  }

  dynamic "retention_policy" {
    for_each = var.retention_days > 0 ? [1] : []
    content {
      retention_period = var.retention_days * 86400
      is_locked        = false # NEVER lock in a lab: locking is irreversible
    }
  }

  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type          = "SetStorageClass"
      storage_class = "NEARLINE"
    }
  }

  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type          = "SetStorageClass"
      storage_class = "COLDLINE"
    }
  }

  lifecycle_rule {
    condition {
      days_since_noncurrent_time = 7
      with_state                 = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.apis]
}
