variable "chart_version" {
  description = "external-secrets Helm chart version"
  type        = string
  default     = "2.10.0"
}

variable "aws_region" {
  description = "Region of the Secrets Manager the ClusterSecretStore reads from"
  type        = string
}

variable "cluster_secret_store_name" {
  description = "Name of the ClusterSecretStore that application ExternalSecrets reference"
  type        = string
  default     = "aws-secrets-manager"
}

variable "app_namespaces" {
  description = "Namespaces allowed to use the ClusterSecretStore"
  type        = list(string)
}
