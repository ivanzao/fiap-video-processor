resource "aws_db_subnet_group" "this" {
  name       = "${var.db_identifier}-subnet-group"
  subnet_ids = var.private_subnet_ids
  tags       = { Name = "${var.db_identifier}-subnet-group" }
}

resource "aws_security_group" "this" {
  name        = "${var.db_identifier}-sg"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = var.vpc_id

  dynamic "ingress" {
    for_each = toset(var.allowed_security_group_ids)
    content {
      description     = "PostgreSQL from ${ingress.value}"
      from_port       = 5432
      to_port         = 5432
      protocol        = "tcp"
      security_groups = [ingress.value]
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.db_identifier}-sg" }
}

resource "aws_db_parameter_group" "this" {
  name        = "${var.db_identifier}-pg16"
  family      = "postgres16"
  description = "Custom parameter group for PostgreSQL 16"

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }
}

resource "aws_db_instance" "this" {
  identifier     = var.db_identifier
  engine         = "postgres"
  engine_version = var.db_engine_version
  instance_class = var.db_instance_class

  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = 100
  storage_type          = "gp3"

  db_name  = var.db_name
  username = var.db_master_username
  password = var.db_master_password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.this.id]
  parameter_group_name   = aws_db_parameter_group.this.name

  publicly_accessible = false
  skip_final_snapshot = var.skip_final_snapshot
  multi_az            = false

  tags = { Name = var.db_identifier }
}

resource "aws_secretsmanager_secret" "app" {
  name                    = "${var.secret_name_prefix}/db-app"
  description             = "PostgreSQL credentials of ${var.db_identifier}"
  recovery_window_in_days = var.secret_recovery_window_days
}

resource "aws_secretsmanager_secret_version" "app" {
  secret_id = aws_secretsmanager_secret.app.id
  secret_string = jsonencode({
    username = var.db_master_username
    password = var.db_master_password
    host     = aws_db_instance.this.address
    port     = aws_db_instance.this.port
    dbname   = var.db_name
    url      = "postgres://${var.db_master_username}:${var.db_master_password}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/${var.db_name}?sslmode=require"
  })
}
