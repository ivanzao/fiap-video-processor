output "namespace" {
  description = "Namespace where the KEDA operator runs"
  value       = helm_release.this.namespace
}
