variable "name" {
  description = "Project name used as ECR pull-through cache prefix, secret suffix and placeholder repository name"
  type        = string
}

variable "environment" {
  description = "Environment name; the pull-through rule and its secret are created per environment"
  type        = string
}

variable "aws_region" {
  description = "AWS region used to build the ECR registry hostname."
  type        = string
}

variable "ghcr_username" {
  description = "GHCR username (GitHub user/org) used for image pulls via ECR Pull Through Cache."
  type        = string
}

variable "ghcr_token" {
  description = "GHCR PAT with read:packages used by ECR PTC to pull private images. Stored in Secrets Manager."
  type        = string
  sensitive   = true
}
