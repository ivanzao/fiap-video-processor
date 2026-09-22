data "aws_caller_identity" "current" {}

resource "aws_secretsmanager_secret" "ghcr" {
  name                    = "ecr-pullthroughcache/${var.name}-${var.environment}"
  description             = "GHCR token used by ECR Pull Through Cache to fetch Lambda images"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "ghcr" {
  secret_id = aws_secretsmanager_secret.ghcr.id
  secret_string = jsonencode({
    username    = var.ghcr_username
    accessToken = var.ghcr_token
  })
}

resource "aws_ecr_pull_through_cache_rule" "ghcr" {
  ecr_repository_prefix = "ghcr-${var.name}-${var.environment}"
  upstream_registry_url = "ghcr.io"
  credential_arn        = aws_secretsmanager_secret.ghcr.arn

  depends_on = [aws_secretsmanager_secret_version.ghcr]
}
