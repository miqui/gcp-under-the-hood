locals {
  service_name = "slo-demo"
  # Filters shared by SLI + log metrics.
  run_filter_base = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${local.service_name}\""
  # Burn rate = (error ratio over window) / (1 - SLO goal). 14.4x for 1h burns
  # 2% of a 30d budget; 6x for 6h burns 5% (Google SRE workbook).
  alert_channels = [for c in google_monitoring_notification_channel.email : c.id]
}

resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "cloudtrace.googleapis.com",
  ])
  service            = each.key
  disable_on_destroy = false
}

# ---------- Workload ----------
resource "google_service_account" "run" {
  account_id   = "slo-demo-run"
  display_name = "SLO demo Cloud Run runtime"
}

# Least privilege: write spans only (logs/metrics are written by the platform).
resource "google_project_iam_member" "trace_agent" {
  project = var.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.run.email}"
}

resource "google_cloud_run_v2_service" "svc" {
  name                = local.service_name
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account = google_service_account.run.email
    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }
    containers {
      image = var.image
      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "FAULT_ERROR_RATE"
        value = tostring(var.fault_error_rate)
      }
      env {
        name  = "FAULT_LATENCY_MS"
        value = tostring(var.fault_latency_ms)
      }
    }
  }

  depends_on = [google_project_service.apis, google_project_iam_member.trace_agent]
}
# No allUsers invoker binding: call with `gcloud auth print-identity-token`.

# ---------- Logging: router, buckets, sinks, exclusions ----------
# Custom bucket receives only app ERROR+ logs, with its own retention.
resource "google_logging_project_bucket_config" "errors" {
  project        = var.project_id
  location       = "global"
  bucket_id      = "slo-demo-errors"
  retention_days = var.log_retention_days
  description    = "ERROR+ logs from the demo service"

  depends_on = [google_project_service.apis]
}

# Sink routes matching entries into the custom bucket. Sinks evaluate
# independently: an entry matching this sink ALSO still goes to _Default
# unless excluded there.
resource "google_logging_project_sink" "errors_to_bucket" {
  name                   = "slo-demo-errors-sink"
  destination            = "logging.googleapis.com/${google_logging_project_bucket_config.errors.id}"
  filter                 = "${local.run_filter_base} AND severity>=ERROR"
  unique_writer_identity = true
}

# Exclusion on the built-in _Default sink: drop health-check request logs
# (excluded entries are not stored in _Default; whether they still feed
# log-based metrics is an exercise in the README - verify it yourself).
resource "google_logging_project_exclusion" "drop_healthz" {
  name        = "exclude-healthz-requests"
  description = "Drop Cloud Run request logs for /healthz from the _Default bucket"
  filter      = "${local.run_filter_base} AND httpRequest.requestUrl:\"/healthz\""

  depends_on = [google_project_service.apis]
}

# ---------- Log-based metrics ----------
resource "google_logging_metric" "injected_errors" {
  name        = "slo_demo_injected_errors"
  description = "Count of ERROR logs with event=injected_error"
  filter      = "${local.run_filter_base} AND jsonPayload.event=\"injected_error\""

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    unit        = "1"
  }
  depends_on = [google_project_service.apis]
}

resource "google_logging_metric" "request_latency" {
  name        = "slo_demo_request_latency_ms"
  description = "Distribution of injected latency from request_ok logs"
  filter      = "${local.run_filter_base} AND jsonPayload.event=\"request_ok\""

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "DISTRIBUTION"
    unit        = "ms"
  }
  value_extractor = "EXTRACT(jsonPayload.latency_ms)"
  bucket_options {
    explicit_buckets {
      bounds = [50, 100, 250, 500, 1000, 2500]
    }
  }
  depends_on = [google_project_service.apis]
}

# ---------- Notification ----------
resource "google_monitoring_notification_channel" "email" {
  count        = var.notification_email == "" ? 0 : 1
  display_name = "slo-demo email"
  type         = "email"
  labels = {
    email_address = var.notification_email
  }
  depends_on = [google_project_service.apis]
}

# ---------- SLO ----------
resource "google_monitoring_custom_service" "svc" {
  service_id   = "slo-demo-svc"
  display_name = "SLO demo (Cloud Run)"
  depends_on   = [google_project_service.apis]
}

resource "google_monitoring_slo" "availability" {
  service             = google_monitoring_custom_service.svc.service_id
  slo_id              = "availability"
  display_name        = "${var.slo_goal * 100}% of requests succeed (non-5xx), rolling ${var.slo_rolling_days}d"
  goal                = var.slo_goal
  rolling_period_days = var.slo_rolling_days

  request_based_sli {
    good_total_ratio {
      total_service_filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.labels.service_name=\"${local.service_name}\"",
      ])
      bad_service_filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.labels.service_name=\"${local.service_name}\"",
        "metric.labels.response_code_class=\"5xx\"",
      ])
    }
  }
}

# ---------- Alerts: multi-window multi-burn-rate ----------
# Fast burn: 14.4x over BOTH 1h and 5m (long window = significance, short
# window = fast reset once the problem stops).
resource "google_monitoring_alert_policy" "fast_burn" {
  display_name = "SLO fast burn (14.4x, 1h AND 5m)"
  combiner     = "AND"
  severity     = "CRITICAL"

  conditions {
    display_name = "burn rate > 14.4 over 1h"
    condition_threshold {
      filter          = "select_slo_burn_rate(\"${google_monitoring_slo.availability.name}\", \"1h\")"
      comparison      = "COMPARISON_GT"
      threshold_value = 14.4
      duration        = "0s"
    }
  }
  conditions {
    display_name = "burn rate > 14.4 over 5m"
    condition_threshold {
      filter          = "select_slo_burn_rate(\"${google_monitoring_slo.availability.name}\", \"5m\")"
      comparison      = "COMPARISON_GT"
      threshold_value = 14.4
      duration        = "0s"
    }
  }

  notification_channels = local.alert_channels
  documentation {
    mime_type = "text/markdown"
    content   = "Error budget is burning ~14.4x too fast (2% of a 30d budget per hour). Page now. Check Logs Explorer for `jsonPayload.event=\"injected_error\"` and Trace for slow spans."
  }
}

# Slow burn: 6x over 6h AND 30m.
resource "google_monitoring_alert_policy" "slow_burn" {
  display_name = "SLO slow burn (6x, 6h AND 30m)"
  combiner     = "AND"
  severity     = "WARNING"

  conditions {
    display_name = "burn rate > 6 over 6h"
    condition_threshold {
      filter          = "select_slo_burn_rate(\"${google_monitoring_slo.availability.name}\", \"6h\")"
      comparison      = "COMPARISON_GT"
      threshold_value = 6
      duration        = "0s"
    }
  }
  conditions {
    display_name = "burn rate > 6 over 30m"
    condition_threshold {
      filter          = "select_slo_burn_rate(\"${google_monitoring_slo.availability.name}\", \"30m\")"
      comparison      = "COMPARISON_GT"
      threshold_value = 6
      duration        = "0s"
    }
  }

  notification_channels = local.alert_channels
  documentation {
    mime_type = "text/markdown"
    content   = "Error budget burning ~6x too fast (5% of a 30d budget in 6h). Ticket-level."
  }
}

# Alert on a log-based metric (shows metric types from logs, not just platform).
resource "google_monitoring_alert_policy" "injected_errors" {
  display_name = "Injected error logs > 5 per minute"
  combiner     = "OR"

  conditions {
    display_name = "log metric rate"
    condition_threshold {
      filter          = "metric.type=\"logging.googleapis.com/user/${google_logging_metric.injected_errors.name}\" AND resource.type=\"cloud_run_revision\""
      comparison      = "COMPARISON_GT"
      threshold_value = 5
      duration        = "60s"
      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }
  notification_channels = local.alert_channels
}

# ---------- Dashboard ----------
resource "google_monitoring_dashboard" "main" {
  dashboard_json = jsonencode({
    displayName = "SLO demo"
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          width = 6, height = 4
          widget = {
            title = "Requests by response class (metric type: run.googleapis.com/request_count)"
            xyChart = {
              dataSets = [{
                plotType = "STACKED_BAR"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/request_count\" resource.type=\"cloud_run_revision\" resource.label.\"service_name\"=\"${local.service_name}\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.label.response_code_class"]
                    }
                  }
                }
              }]
            }
          }
        },
        {
          xPos = 6, width = 6, height = 4
          widget = {
            title = "Error ratio (PromQL)"
            xyChart = {
              dataSets = [{
                plotType = "LINE"
                timeSeriesQuery = {
                  prometheusQuery = "sum(rate(run_googleapis_com:request_count{service_name=\"${local.service_name}\",response_code_class=\"5xx\"}[5m])) / sum(rate(run_googleapis_com:request_count{service_name=\"${local.service_name}\"}[5m]))"
                }
              }]
            }
          }
        },
        {
          yPos = 4, width = 6, height = 4
          widget = {
            title = "Request latency p95 (ms)"
            xyChart = {
              dataSets = [{
                plotType = "LINE"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/request_latencies\" resource.type=\"cloud_run_revision\" resource.label.\"service_name\"=\"${local.service_name}\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_MAX"
                    }
                  }
                }
              }]
            }
          }
        },
        {
          xPos = 6, yPos = 4, width = 6, height = 4
          widget = {
            title = "Injected errors (log-based metric)"
            xyChart = {
              dataSets = [{
                plotType = "LINE"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"logging.googleapis.com/user/${google_logging_metric.injected_errors.name}\""
                    aggregation = {
                      alignmentPeriod  = "60s"
                      perSeriesAligner = "ALIGN_SUM"
                    }
                  }
                }
              }]
            }
          }
        },
      ]
    }
  })

  depends_on = [google_project_service.apis]
}
