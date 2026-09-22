output "bucket_name" {
  description = "Bucket name"
  value       = var.bucket_name
}

output "bucket_arn" {
  description = "Bucket ARN"
  value       = "arn:aws:s3:::${var.bucket_name}"
}
