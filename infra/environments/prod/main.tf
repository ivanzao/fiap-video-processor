module "infra" {
  source = "../../modules"

  environment          = var.environment
  aws_region           = var.aws_region
  vpc_cidr             = var.vpc_cidr
  node_desired_size    = var.node_desired_size
  node_min_size        = var.node_min_size
  node_max_size        = var.node_max_size
  cors_allowed_origins = var.cors_allowed_origins
  manage_buckets       = var.manage_buckets
  ghcr_token           = var.ghcr_token
  mailersend_token     = var.mailersend_token
  mail_from            = var.mail_from
}
