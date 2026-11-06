output "instance_connection_name" {
  value = google_sql_database_instance.pg.connection_name
}

output "db_user" {
  value = google_sql_user.app_iam.name
}

output "db_name" {
  value = google_sql_database.app.name
}

output "bucket" {
  value = google_storage_bucket.data.name
}

output "app_service_account" {
  value = google_service_account.app.email
}

output "instance_name" {
  value = google_sql_database_instance.pg.name
}
