variable "bucket_name" {
  description = "Globally unique S3 bucket name"
  type        = string
}

variable "cors_allowed_origins" {
  description = "Origins allowed to PUT/GET objects directly from the browser"
  type        = list(string)
  default     = ["*"]
}

variable "expiration_rules" {
  description = "Object expiration in days keyed by key prefix (e.g. { \"uploads/\" = 7 })"
  type        = map(number)
  default     = {}
}

variable "manage_bucket" {
  description = "Create the bucket with Terraform. Set to false where the account policy keeps the provider from reading buckets (AWS Academy); the bucket must then exist before apply and only its configuration is managed here"
  type        = bool
  default     = true
}

variable "force_destroy" {
  description = "Delete all objects when the bucket is destroyed"
  type        = bool
  default     = true
}
