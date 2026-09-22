output "lambda_image_base" {
  description = "Base ECR path (GHCR via pull-through cache) that app CI targets when publishing real Lambda images"
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${aws_ecr_pull_through_cache_rule.ghcr.ecr_repository_prefix}/${var.ghcr_username}"
}

output "lambda_placeholder_image" {
  description = "Placeholder arm64 image the pipeline creates and seeds in the account-wide repository before the full apply; Lambdas create against it so a greenfield apply never depends on app repos having published. App CI overrides the real image via update-function-code (image_uri is ignored)."
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${var.name}-lambda-placeholder:bootstrap"
}
