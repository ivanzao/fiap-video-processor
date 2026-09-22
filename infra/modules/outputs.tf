output "namespace" {
  description = "Kubernetes namespace of the environment"
  value       = kubernetes_namespace.this.metadata[0].name
}

output "apigw_endpoint" {
  description = "Public base URL of the environment"
  value       = module.gateway.invoke_url
}

output "bucket_name" {
  description = "Video bucket name"
  value       = module.storage.bucket_name
}

output "node_ports" {
  description = "NodePort per HTTP service"
  value       = { for name, ports in local.http_services : name => ports.node_port }
}

output "ssm_parameter_names" {
  description = "Every SSM parameter published for the deploy pipeline"
  value       = sort([for p in aws_ssm_parameter.this : p.name])
}

output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API server endpoint"
  value       = module.eks.cluster_endpoint
}

output "cluster_ca" {
  description = "Base64-encoded cluster CA certificate"
  value       = module.eks.cluster_ca
}
