output "endpoint" {
  description = "RDS instance hostname"
  value       = aws_db_instance.this.address
}

output "port" {
  description = "RDS instance port"
  value       = aws_db_instance.this.port
}

output "secret_arn" {
  description = "ARN of the service credentials secret (JSON: username, password, host, port, dbname, url)"
  value       = aws_secretsmanager_secret.app.arn
}

output "url" {
  description = "PostgreSQL connection URL of the service"
  value       = "postgres://${var.db_master_username}:${var.db_master_password}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/${var.db_name}?sslmode=require"
  sensitive   = true
}
