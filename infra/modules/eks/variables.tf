variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
}

variable "cluster_role_arn" {
  description = "IAM role assumed by the EKS control plane"
  type        = string
}

variable "node_role_arn" {
  description = "IAM role assumed by the worker nodes"
  type        = string
}

variable "cluster_version" {
  description = "Kubernetes version of the EKS control plane"
  type        = string
  default     = "1.34"
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for EKS node groups"
  type        = list(string)
}

variable "public_subnet_ids" {
  description = "Public subnet IDs for the EKS control plane"
  type        = list(string)
}

variable "node_instance_type" {
  description = "EC2 instance type for worker nodes"
  type        = string
  default     = "t3.medium"
}

variable "node_desired_size" {
  description = "Desired number of worker nodes"
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Minimum number of worker nodes"
  type        = number
  default     = 1
}

variable "node_max_size" {
  description = "Maximum number of worker nodes"
  type        = number
  default     = 4
}

variable "public_access_cidrs" {
  description = "CIDR blocks allowed to reach the EKS public API endpoint"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
