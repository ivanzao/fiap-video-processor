output "cluster_name" {
  description = "EKS cluster name"
  value       = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  description = "EKS cluster API server endpoint"
  value       = aws_eks_cluster.this.endpoint
}

output "cluster_ca" {
  description = "Base64-encoded cluster CA certificate"
  value       = aws_eks_cluster.this.certificate_authority[0].data
}

output "cluster_sg_id" {
  description = "EKS cluster security group ID"
  value       = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id
}

output "node_group_asg_names" {
  description = "ASG names for the default node group, used for NLB target registration"
  value       = flatten([for r in aws_eks_node_group.this.resources : r.autoscaling_groups[*].name])
}
