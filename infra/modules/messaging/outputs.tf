output "topic_arns" {
  description = "Events topic ARN per service"
  value       = { for name, topic in aws_sns_topic.events : name => topic.arn }
}

output "queue_urls" {
  description = "Inbox queue URL per consuming service"
  value       = { for name, queue in aws_sqs_queue.inbox : name => queue.url }
}

output "queue_arns" {
  description = "Inbox queue ARN per consuming service"
  value       = { for name, queue in aws_sqs_queue.inbox : name => queue.arn }
}

output "queue_names" {
  description = "Inbox queue name per consuming service"
  value       = { for name, queue in aws_sqs_queue.inbox : name => queue.name }
}
