variable "db_identifier" {
  description = "RDS instance identifier"
  type        = string
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "Initial allocated storage in GB"
  type        = number
  default     = 20
}

variable "db_engine_version" {
  description = "PostgreSQL engine version"
  type        = string
  default     = "16.14"
}

variable "db_name" {
  description = "Database created on the instance and owned by the service"
  type        = string
}

variable "db_master_username" {
  description = "Master database username, used by the service as its own user"
  type        = string
}

variable "db_master_password" {
  description = "Master database password"
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "VPC ID for the RDS subnet group and security group"
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for the DB subnet group"
  type        = list(string)
}

variable "allowed_security_group_ids" {
  description = "Security groups allowed to reach PostgreSQL (EKS cluster, Lambda)"
  type        = list(string)
}

variable "skip_final_snapshot" {
  description = "Skip final snapshot on destroy"
  type        = bool
  default     = true
}

variable "secret_recovery_window_days" {
  description = "Days before a deleted Secrets Manager secret is permanently removed"
  type        = number
  default     = 0
}

variable "secret_name_prefix" {
  description = "Prefix of the Secrets Manager entry created by this module, e.g. video-processor/staging/api"
  type        = string
}
