variable "name" {
  description = "Project name used as resource name prefix"
  type        = string
}

variable "environment" {
  description = "Environment name used as resource name suffix"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs used by the VPC Link and the NLB"
  type        = list(string)
}

variable "vpc_link_sg_id" {
  description = "Security group applied to the VPC Link"
  type        = string
}

variable "eks_cluster_sg_id" {
  description = "EKS cluster security group that receives NodePort traffic from the NLB"
  type        = string
}

variable "node_group_asg_names" {
  description = "EKS node group Auto Scaling Group names attached to the target groups"
  type        = list(string)
}

variable "http_services" {
  description = "In-cluster HTTP services exposed through the NLB, keyed by service name"
  type = map(object({
    node_port     = number
    listener_port = number
  }))
}

variable "authorizer" {
  description = "Lambda REQUEST authorizer that validates the Bearer token"
  type = object({
    invoke_arn    = string
    function_name = string
  })
}

variable "lambda_routes" {
  description = "Public routes served directly by Lambda functions (signup, login)"
  type = map(object({
    route_key     = string
    invoke_arn    = string
    function_name = string
  }))
  default = {}
}

variable "protected_routes" {
  description = "Routes that require the authorizer and forward to an in-cluster service with identity headers injected"
  type = map(object({
    route_key = string
    service   = string
  }))
}

variable "public_routes" {
  description = "Routes forwarded to an in-cluster service without authorization; identity headers sent by the client are stripped"
  type = map(object({
    route_key = string
    service   = string
  }))
  default = {}
}

variable "identity_headers" {
  description = "Headers injected on protected routes from the authorizer context, e.g. { X-User-Id = \"$context.authorizer.userId\" }"
  type        = map(string)
}
