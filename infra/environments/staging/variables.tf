variable "environment" {
  description = "Environment name"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "CIDR block of this environment's VPC"
  type        = string
}

variable "node_desired_size" {
  description = "Desired number of EKS worker nodes"
  type        = number
}

variable "node_min_size" {
  description = "Minimum number of EKS worker nodes"
  type        = number
}

variable "node_max_size" {
  description = "Maximum number of EKS worker nodes"
  type        = number
}

variable "manage_buckets" {
  description = "Create S3 buckets with Terraform; false on AWS Academy, where the pipeline creates them before apply"
  type        = bool
  default     = true
}

variable "cors_allowed_origins" {
  description = "Origins allowed to upload directly to the bucket"
  type        = list(string)
  default     = ["*"]
}

variable "ghcr_token" {
  description = "GHCR token (read:packages) used by the ECR pull-through cache"
  type        = string
  sensitive   = true
}

variable "mailersend_token" {
  description = "MailerSend API token used by the worker"
  type        = string
  sensitive   = true
}

variable "mail_from" {
  description = "Sender address of the notification emails; on a MailerSend trial it is the trial-domain address tied to the token, so both change together"
  type        = string
}
