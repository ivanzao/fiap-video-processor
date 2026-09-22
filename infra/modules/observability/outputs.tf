output "otlp_http_endpoint" {
  description = "In-cluster OTLP/HTTP endpoint applications export traces to"
  value       = local.otlp_endpoint
}

output "namespace" {
  description = "Observability namespace"
  value       = kubernetes_namespace.this.metadata[0].name
}
