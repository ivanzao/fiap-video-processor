variable "aws_region" {
  description = "Region of the Tempo trace bucket"
  type        = string
}

variable "tempo_bucket_name" {
  description = "Globally unique bucket name for Tempo trace blocks"
  type        = string
}

variable "manage_bucket" {
  description = "Create the Tempo bucket with Terraform. Set to false where the account policy keeps the provider from reading buckets (AWS Academy); the bucket must then exist before apply"
  type        = bool
  default     = true
}

variable "tempo_bucket_retention_days" {
  description = "Days before trace blocks expire in S3"
  type        = number
  default     = 7
}

variable "tempo_retention" {
  description = "Tempo block retention (Go duration)"
  type        = string
  default     = "72h"
}

variable "prometheus_retention" {
  description = "Prometheus TSDB retention"
  type        = string
  default     = "7d"
}

variable "grafana_admin_password" {
  description = "Grafana admin UI password"
  type        = string
  default     = "admin"
}

variable "kube_prometheus_stack_version" {
  description = "kube-prometheus-stack Helm chart version"
  type        = string
  default     = "90.2.0"
}

variable "tempo_version" {
  description = "Tempo Helm chart version"
  type        = string
  default     = "1.24.4"
}

variable "alloy_version" {
  description = "Alloy Helm chart version"
  type        = string
  default     = "1.12.1"
}

variable "grafana_folder" {
  description = "Grafana folder that receives the provisioned dashboards"
  type        = string
  default     = "Video Processor"
}
