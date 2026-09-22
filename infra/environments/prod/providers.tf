provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Application = "video-processor"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

provider "kubernetes" {
  host                   = module.infra.cluster_endpoint
  cluster_ca_certificate = base64decode(module.infra.cluster_ca)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.infra.cluster_name, "--region", var.aws_region]
  }
}

provider "helm" {
  kubernetes = {
    host                   = module.infra.cluster_endpoint
    cluster_ca_certificate = base64decode(module.infra.cluster_ca)
    exec = {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", module.infra.cluster_name, "--region", var.aws_region]
    }
  }
}
