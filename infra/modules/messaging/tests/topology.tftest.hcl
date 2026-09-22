mock_provider "aws" {}

variables {
  name        = "video-processor"
  environment = "test"
  services = {
    api    = { subscribes_to = ["worker"], visibility_timeout_seconds = 30 }
    worker = { subscribes_to = ["api"], visibility_timeout_seconds = 300 }
    auth   = {}
  }
}

run "every_service_gets_a_topic_and_only_consumers_get_queues" {
  command = plan

  assert {
    condition     = length(aws_sns_topic.events) == 3
    error_message = "expected one events topic per service"
  }

  assert {
    condition     = length(aws_sqs_queue.inbox) == 2 && !contains(keys(aws_sqs_queue.inbox), "auth")
    error_message = "only api and worker subscribe, so only they get inbox queues"
  }

  assert {
    condition     = length(aws_sqs_queue.dlq) == 2
    error_message = "every inbox queue has a dead-letter queue"
  }

  assert {
    condition     = toset(keys(aws_sns_topic_subscription.inbox)) == toset(["api-from-worker", "worker-from-api"])
    error_message = "subscriptions must wire each consumer to the topics it subscribes to"
  }

  assert {
    condition     = aws_sqs_queue.inbox["worker"].visibility_timeout_seconds == 300
    error_message = "the worker inbox keeps its long visibility timeout"
  }

  assert {
    condition     = aws_sqs_queue.inbox["api"].name == "video-processor-api-inbox-test" && aws_sns_topic.events["worker"].name == "video-processor-worker-events-test"
    error_message = "names follow <name>-<service>-<kind>-<environment>"
  }
}

run "rejects_subscription_to_unknown_service" {
  command = plan

  variables {
    services = {
      api = { subscribes_to = ["ghost"] }
    }
  }

  expect_failures = [var.services]
}
