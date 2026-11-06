data "google_project" "this" {
  project_id = var.project_id
}

resource "google_project_service" "pubsub" {
  project            = var.project_id
  service            = "pubsub.googleapis.com"
  disable_on_destroy = false
}

# Topic-level retention lets you seek a subscription back in time even for
# messages that were already acked.
resource "google_pubsub_topic" "main" {
  name                       = "${var.name_prefix}-events"
  message_retention_duration = "86400s"

  depends_on = [google_project_service.pubsub]
}

resource "google_pubsub_topic" "dead_letter" {
  name = "${var.name_prefix}-events-dead-letter"

  depends_on = [google_project_service.pubsub]
}

# Pull subscription with a SHORT ack deadline (10s) so a slow handler causes
# redeliveries, plus retry backoff and a dead-letter policy.
resource "google_pubsub_subscription" "pull" {
  name                       = "${var.name_prefix}-pull"
  topic                      = google_pubsub_topic.main.id
  ack_deadline_seconds       = 10
  message_retention_duration = "604800s"
  retain_acked_messages      = true # allows seek-to-timestamp replay of acked messages

  retry_policy {
    minimum_backoff = "5s"
    maximum_backoff = "60s"
  }

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.dead_letter.id
    max_delivery_attempts = 5
  }

  expiration_policy {
    ttl = "" # never expire
  }
}

# Ordered subscription: pairs with ordering keys on the publisher.
resource "google_pubsub_subscription" "ordered" {
  name                    = "${var.name_prefix}-ordered"
  topic                   = google_pubsub_topic.main.id
  ack_deadline_seconds    = 20
  enable_message_ordering = true

  expiration_policy {
    ttl = ""
  }
}

# Exactly-once delivery: acks are confirmed; ack IDs can expire, so clients must use AckWithResult.
resource "google_pubsub_subscription" "exactly_once" {
  name                         = "${var.name_prefix}-eod"
  topic                        = google_pubsub_topic.main.id
  ack_deadline_seconds         = 30
  enable_exactly_once_delivery = true

  expiration_policy {
    ttl = ""
  }
}

# Receives dead-lettered messages so you can inspect them.
resource "google_pubsub_subscription" "dead_letter_sink" {
  name                 = "${var.name_prefix}-dlq-sink"
  topic                = google_pubsub_topic.dead_letter.id
  ack_deadline_seconds = 20

  expiration_policy {
    ttl = ""
  }
}

# Optional push subscription (only created if an endpoint is supplied).
resource "google_pubsub_subscription" "push" {
  count                = var.push_endpoint == "" ? 0 : 1
  name                 = "${var.name_prefix}-push"
  topic                = google_pubsub_topic.main.id
  ack_deadline_seconds = 10

  push_config {
    push_endpoint = var.push_endpoint
  }

  expiration_policy {
    ttl = ""
  }
}

# Dead lettering requires the Pub/Sub service agent to publish to the DLQ
# topic and to ack/nack on the source subscription.
locals {
  pubsub_agent = "serviceAccount:service-${data.google_project.this.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

resource "google_pubsub_topic_iam_member" "dlq_publisher" {
  topic  = google_pubsub_topic.dead_letter.name
  role   = "roles/pubsub.publisher"
  member = local.pubsub_agent
}

resource "google_pubsub_subscription_iam_member" "dlq_subscriber" {
  subscription = google_pubsub_subscription.pull.name
  role         = "roles/pubsub.subscriber"
  member       = local.pubsub_agent
}
