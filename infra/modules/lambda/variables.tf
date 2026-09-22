variable "name" {
  description = "Project name used as function name prefix"
  type        = string
}

variable "environment" {
  description = "Environment name used as function name suffix"
  type        = string
}

variable "functions" {
  description = "Lambda functions to create, keyed by short name (matches the cmd/<name> binary of the auth service)"
  type = map(object({
    timeout     = optional(number, 15)
    memory_size = optional(number, 256)
    environment = optional(map(string), {})
  }))
}

variable "image_uri" {
  description = "Container image used at create time. App CI publishes the real image with update-function-code, so later changes are ignored."
  type        = string
}

variable "role_arn" {
  description = "Execution role ARN (LabRole on AWS Academy)"
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for the function ENIs"
  type        = list(string)
}

variable "security_group_ids" {
  description = "Security groups attached to the function ENIs"
  type        = list(string)
}
