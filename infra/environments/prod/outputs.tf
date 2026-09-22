output "apigw_endpoint" {
  description = "Public base URL of the environment"
  value       = module.infra.apigw_endpoint
}

output "namespace" {
  description = "Kubernetes namespace of the environment"
  value       = module.infra.namespace
}

output "node_ports" {
  description = "NodePort per HTTP service"
  value       = module.infra.node_ports
}

output "cluster_name" {
  description = "EKS cluster name of the environment"
  value       = module.infra.cluster_name
}
