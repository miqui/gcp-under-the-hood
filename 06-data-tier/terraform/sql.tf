resource "random_id" "suffix" {
  byte_length = 3 # Cloud SQL instance names are reserved ~1 week after delete
}

resource "google_sql_database_instance" "pg" {
  name                = "${var.name_prefix}-pg-${random_id.suffix.hex}"
  region              = var.region
  database_version    = "POSTGRES_15"
  deletion_protection = var.db_deletion_protection

  settings {
    tier              = var.db_tier
    edition           = "ENTERPRISE"
    availability_type = var.ha_enabled ? "REGIONAL" : "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 10

    ip_configuration {
      ipv4_enabled                                  = false # no public IP
      private_network                               = google_compute_network.vpc.id
      enable_private_path_for_google_cloud_services = false
    }

    database_flags {
      name  = "cloudsql.iam_authentication"
      value = "on"
    }

    backup_configuration {
      enabled                        = true
      start_time                     = "03:00"
      point_in_time_recovery_enabled = true # ships WAL to GCS continuously
      transaction_log_retention_days = 7
      backup_retention_settings {
        retained_backups = 7
      }
    }
  }

  depends_on = [google_service_networking_connection.psa]
}

resource "google_sql_database" "app" {
  name     = "appdb"
  instance = google_sql_database_instance.pg.name
}

# IAM DB auth: no password anywhere. Postgres user name = SA email minus ".gserviceaccount.com".
resource "google_sql_user" "app_iam" {
  name     = trimsuffix(google_service_account.app.email, ".gserviceaccount.com")
  instance = google_sql_database_instance.pg.name
  type     = "CLOUD_IAM_SERVICE_ACCOUNT"
}
