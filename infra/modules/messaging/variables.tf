variable "name" {
  description = "Project name used as resource name prefix"
  type        = string
}

variable "environment" {
  description = "Environment name used as resource name suffix"
  type        = string
}

variable "services" {
  description = "Services on the event bus. Every service gets an events topic; services that subscribe to others get an inbox queue with a dead-letter queue."
  type = map(object({
    subscribes_to              = optional(set(string), [])
    visibility_timeout_seconds = optional(number, 30)
    max_receive_count          = optional(number, 3)
  }))

  validation {
    condition     = alltrue([for name, svc in var.services : alltrue([for topic in svc.subscribes_to : contains(keys(var.services), topic)])])
    error_message = "Every entry in subscribes_to must be the name of another service in the map."
  }
}
