output "topic" {
  value = google_pubsub_topic.main.name
}

output "dead_letter_topic" {
  value = google_pubsub_topic.dead_letter.name
}

output "subscriptions" {
  value = {
    pull         = google_pubsub_subscription.pull.name
    ordered      = google_pubsub_subscription.ordered.name
    exactly_once = google_pubsub_subscription.exactly_once.name
    dlq_sink     = google_pubsub_subscription.dead_letter_sink.name
  }
}
