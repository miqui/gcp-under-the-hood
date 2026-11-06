locals {
  # Same schema for all three tables so only the physical layout differs.
  trips_schema = jsonencode([
    { name = "trip_id", type = "STRING", mode = "NULLABLE" },
    { name = "subscriber_type", type = "STRING", mode = "NULLABLE" },
    { name = "start_time", type = "TIMESTAMP", mode = "NULLABLE" },
    { name = "start_station_id", type = "INTEGER", mode = "NULLABLE" },
    { name = "start_station_name", type = "STRING", mode = "NULLABLE" },
    { name = "end_station_id", type = "STRING", mode = "NULLABLE" },
    { name = "duration_minutes", type = "INTEGER", mode = "NULLABLE" },
  ])
}

resource "google_bigquery_dataset" "lab" {
  dataset_id    = var.dataset_id
  location      = var.region
  friendly_name = "BigQuery internals lab"
  description   = "Lab dataset for gcp-under-the-hood/08-bigquery-internals"

  # Lab only: lets `terraform destroy` remove tables the Go tool loaded.
  delete_contents_on_destroy = true
}

# 1. Flat table: no partitioning, no clustering (baseline).
resource "google_bigquery_table" "trips_flat" {
  dataset_id          = google_bigquery_dataset.lab.dataset_id
  table_id            = "trips_flat"
  schema              = local.trips_schema
  deletion_protection = false
}

# 2. Partitioned by day on start_time (partition pruning).
resource "google_bigquery_table" "trips_part" {
  dataset_id          = google_bigquery_dataset.lab.dataset_id
  table_id            = "trips_part"
  schema              = local.trips_schema
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "start_time"
  }
}

# 3. Partitioned + clustered (block pruning inside partitions).
resource "google_bigquery_table" "trips_part_clust" {
  dataset_id          = google_bigquery_dataset.lab.dataset_id
  table_id            = "trips_part_clust"
  schema              = local.trips_schema
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "start_time"
  }

  clustering = ["subscriber_type", "start_station_id"]
}

# Materialized view: precomputed aggregate, incrementally maintained;
# the optimizer can transparently rewrite matching queries to use it.
resource "google_bigquery_table" "trips_daily_mv" {
  dataset_id          = google_bigquery_dataset.lab.dataset_id
  table_id            = "trips_daily_mv"
  deletion_protection = false

  materialized_view {
    enable_refresh      = true
    refresh_interval_ms = 1800000
    query               = <<-SQL
      SELECT
        DATE(start_time) AS trip_date,
        subscriber_type,
        COUNT(*) AS trips,
        SUM(duration_minutes) AS total_minutes
      FROM `${var.project_id}.${google_bigquery_dataset.lab.dataset_id}.${google_bigquery_table.trips_part_clust.table_id}`
      GROUP BY trip_date, subscriber_type
    SQL
  }
}

# Streaming target for the Storage Write API demo is trips_flat-shaped;
# on-demand pricing is the default, so no reservation is created (a
# reservation costs money per slot-hour; see README for the capacity model).
