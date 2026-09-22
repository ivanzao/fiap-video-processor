locals {
  consumers = { for name, svc in var.services : name => svc if length(svc.subscribes_to) > 0 }

  subscriptions = merge([
    for name, svc in local.consumers : {
      for topic in svc.subscribes_to : "${name}-from-${topic}" => { queue = name, topic = topic }
    }
  ]...)
}

resource "aws_sns_topic" "events" {
  for_each = var.services

  name = "${var.name}-${each.key}-events-${var.environment}"
}

resource "aws_sqs_queue" "dlq" {
  for_each = local.consumers

  name                      = "${var.name}-${each.key}-inbox-dlq-${var.environment}"
  message_retention_seconds = 1209600
}

resource "aws_sqs_queue" "inbox" {
  for_each = local.consumers

  name                       = "${var.name}-${each.key}-inbox-${var.environment}"
  visibility_timeout_seconds = each.value.visibility_timeout_seconds

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[each.key].arn
    maxReceiveCount     = each.value.max_receive_count
  })
}

resource "aws_sns_topic_subscription" "inbox" {
  for_each = local.subscriptions

  topic_arn            = aws_sns_topic.events[each.value.topic].arn
  protocol             = "sqs"
  endpoint             = aws_sqs_queue.inbox[each.value.queue].arn
  raw_message_delivery = true
}

data "aws_iam_policy_document" "inbox" {
  for_each = local.consumers

  statement {
    effect    = "Allow"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.inbox[each.key].arn]

    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }

    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = [for topic in each.value.subscribes_to : aws_sns_topic.events[topic].arn]
    }
  }
}

resource "aws_sqs_queue_policy" "inbox" {
  for_each = local.consumers

  queue_url = aws_sqs_queue.inbox[each.key].id
  policy    = data.aws_iam_policy_document.inbox[each.key].json
}
