variable "name" {
  description = "Project name used as prefix for every resource"
  type        = string
  default     = "video-processor"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (staging, prod)"
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9]{1,15}$", var.environment))
    error_message = "Environment must be a short lowercase identifier."
  }
}

variable "db_instance_class" {
  description = "RDS instance class of each service database"
  type        = string
  default     = "db.t3.micro"
}

variable "manage_buckets" {
  description = "Create S3 buckets with Terraform. False on AWS Academy, whose service control policy denies s3:GetBucketObjectLockConfiguration and makes the provider fail to read any bucket; the pipeline then creates the buckets before apply"
  type        = bool
  default     = true
}

variable "cors_allowed_origins" {
  description = "Origins allowed to upload directly to the bucket"
  type        = list(string)
  default     = ["*"]
}

variable "upload_retention_days" {
  description = "Days before raw uploads expire from the bucket"
  type        = number
  default     = 7
}

variable "token_ttl" {
  description = "JWT lifetime issued by the login Lambda (Go duration)"
  type        = string
  default     = "12h"
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

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "cluster_version" {
  description = "Kubernetes version of the EKS control plane"
  type        = string
  default     = "1.34"
}

variable "node_instance_type" {
  description = "EC2 instance type for EKS worker nodes"
  type        = string
  default     = "t3.medium"
}

variable "node_desired_size" {
  description = "Desired number of EKS worker nodes"
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Minimum number of EKS worker nodes"
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Maximum number of EKS worker nodes"
  type        = number
  default     = 4
}

variable "ghcr_username" {
  description = "GitHub user or org that publishes the Lambda images"
  type        = string
  default     = "ivanzao"
}

variable "ghcr_token" {
  description = "GHCR token (read:packages) used by the ECR pull-through cache"
  type        = string
  sensitive   = true
}
