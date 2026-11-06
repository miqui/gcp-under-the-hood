output "dataset_id" {
  value = google_bigquery_dataset.lab.dataset_id
}

output "tables" {
  value = {
    flat           = google_bigquery_table.trips_flat.table_id
    partitioned    = google_bigquery_table.trips_part.table_id
    part_clustered = google_bigquery_table.trips_part_clust.table_id
    mat_view       = google_bigquery_table.trips_daily_mv.table_id
  }
}
