variable "name" {
  description = "Name prefix used for all resources in this module"
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
}

variable "cluster_name" {
  description = "EKS cluster name, used for kubernetes.io/cluster/* subnet discovery tags"
  type        = string
}
