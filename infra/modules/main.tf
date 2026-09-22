data "aws_caller_identity" "current" {}

module "vpc" {
  source       = "./vpc"
  name         = "${local.name}-${var.environment}"
  vpc_cidr     = var.vpc_cidr
  cluster_name = local.cluster_name
}

module "eks" {
  source             = "./eks"
  cluster_name       = local.cluster_name
  cluster_version    = var.cluster_version
  cluster_role_arn   = local.lab_role_arn
  node_role_arn      = local.lab_role_arn
  private_subnet_ids = module.vpc.private_subnet_ids
  public_subnet_ids  = module.vpc.public_subnet_ids
  node_instance_type = var.node_instance_type
  node_desired_size  = var.node_desired_size
  node_min_size      = var.node_min_size
  node_max_size      = var.node_max_size
}

module "registry" {
  source        = "./registry"
  name          = local.name
  environment   = var.environment
  aws_region    = var.aws_region
  ghcr_username = var.ghcr_username
  ghcr_token    = var.ghcr_token
}

module "external_secrets" {
  source         = "./external-secrets"
  aws_region     = var.aws_region
  app_namespaces = [local.namespace]

  depends_on = [module.eks]
}

module "keda" {
  source = "./keda"

  depends_on = [module.eks]
}

module "observability" {
  source            = "./observability"
  aws_region        = var.aws_region
  tempo_bucket_name = "${local.name}-tempo-${var.environment}-${local.account_id}"
  manage_bucket     = var.manage_buckets

  depends_on = [module.eks]
}

resource "kubernetes_namespace" "this" {
  metadata {
    name   = local.namespace
    labels = { environment = var.environment, app = local.name }
  }

  depends_on = [module.eks]
}

resource "random_password" "db" {
  for_each = local.databases

  length  = 32
  special = false
}

module "rds" {
  source   = "./rds"
  for_each = local.databases

  db_identifier              = "${local.name}-${each.key}-${var.environment}-db"
  db_name                    = each.value.db_name
  db_master_username         = each.value.role
  db_master_password         = random_password.db[each.key].result
  db_instance_class          = var.db_instance_class
  vpc_id                     = module.vpc.vpc_id
  private_subnet_ids         = module.vpc.private_subnet_ids
  allowed_security_group_ids = [module.eks.cluster_sg_id, module.vpc.lambda_sg_id]
  secret_name_prefix         = "${local.name}/${var.environment}/${each.key}"
}

module "storage" {
  source               = "./storage"
  bucket_name          = "${local.name}-${var.environment}-${local.account_id}"
  cors_allowed_origins = var.cors_allowed_origins
  expiration_rules     = { "uploads/" = var.upload_retention_days }
  manage_bucket        = var.manage_buckets
}

module "messaging" {
  source      = "./messaging"
  name        = local.name
  environment = var.environment
  services    = local.messaging_services
}

resource "random_password" "jwt_secret" {
  length  = 64
  special = false
}

locals {
  auth_lambda_env = {
    DATABASE_URL = module.rds["auth"].url
    JWT_SECRET   = random_password.jwt_secret.result
    TOKEN_TTL    = var.token_ttl
  }
}

module "lambda" {
  source             = "./lambda"
  name               = local.name
  environment        = var.environment
  image_uri          = module.registry.lambda_placeholder_image
  role_arn           = local.lab_role_arn
  private_subnet_ids = module.vpc.private_subnet_ids
  security_group_ids = [module.vpc.lambda_sg_id]

  functions = {
    signup = { environment = local.auth_lambda_env }
    login  = { environment = local.auth_lambda_env }
    authorizer = {
      timeout     = 5
      memory_size = 128
      environment = { JWT_SECRET = random_password.jwt_secret.result }
    }
  }
}

module "gateway" {
  source               = "./gateway"
  name                 = local.name
  environment          = var.environment
  vpc_id               = module.vpc.vpc_id
  private_subnet_ids   = module.vpc.private_subnet_ids
  vpc_link_sg_id       = module.vpc.lambda_sg_id
  eks_cluster_sg_id    = module.eks.cluster_sg_id
  node_group_asg_names = module.eks.node_group_asg_names
  http_services        = local.http_services
  identity_headers     = local.identity_headers

  authorizer = {
    invoke_arn    = module.lambda.functions["authorizer"].invoke_arn
    function_name = module.lambda.functions["authorizer"].name
  }

  lambda_routes = {
    signup = { route_key = "POST /auth/v1/signup", invoke_arn = module.lambda.functions["signup"].invoke_arn, function_name = module.lambda.functions["signup"].name }
    login  = { route_key = "POST /auth/v1/login", invoke_arn = module.lambda.functions["login"].invoke_arn, function_name = module.lambda.functions["login"].name }
  }

  protected_routes = {
    api_v1 = { route_key = "ANY /v1/{proxy+}", service = "api" }
  }

  public_routes = {
    root   = { route_key = "GET /", service = "api" }
    assets = { route_key = "GET /{proxy+}", service = "api" }
    health = { route_key = "GET /health", service = "api" }
  }
}

resource "aws_secretsmanager_secret" "mailersend" {
  name                    = "${local.name}/${var.environment}/worker/mailersend"
  description             = "MailerSend API token used by the worker to send notifications"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "mailersend" {
  secret_id     = aws_secretsmanager_secret.mailersend.id
  secret_string = jsonencode({ token = var.mailersend_token, from = var.mail_from })
}
