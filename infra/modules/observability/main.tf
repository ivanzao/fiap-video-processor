locals {
  namespace     = "observability"
  otlp_endpoint = "http://alloy-gateway.${local.namespace}.svc.cluster.local:4317"
}

resource "kubernetes_namespace" "this" {
  metadata {
    name   = local.namespace
    labels = { app = "observability" }
  }
}

resource "aws_s3_bucket" "this" {
  count = var.manage_bucket ? 1 : 0

  bucket        = var.tempo_bucket_name
  force_destroy = true
}

resource "aws_s3_bucket_lifecycle_configuration" "this" {
  bucket = var.tempo_bucket_name

  depends_on = [aws_s3_bucket.this]

  rule {
    id     = "expire-traces"
    status = "Enabled"

    filter {}

    expiration {
      days = var.tempo_bucket_retention_days
    }
  }
}

resource "helm_release" "kube_prometheus_stack" {
  name       = "kube-prometheus-stack"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = var.kube_prometheus_stack_version
  namespace  = kubernetes_namespace.this.metadata[0].name
  timeout    = 1200
  wait       = false

  values = [
    templatefile("${path.module}/helm-values/kube-prometheus-stack.yaml", {
      grafana_admin_password = var.grafana_admin_password
      prometheus_retention   = var.prometheus_retention
    }),
    file("${path.module}/helm-values/alerts.yaml"),
  ]
}

resource "helm_release" "tempo" {
  name       = "tempo"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "tempo"
  version    = var.tempo_version
  namespace  = kubernetes_namespace.this.metadata[0].name
  timeout    = 300
  wait       = false

  values = [
    templatefile("${path.module}/helm-values/tempo.yaml", {
      aws_region      = var.aws_region
      tempo_bucket    = var.tempo_bucket_name
      tempo_retention = var.tempo_retention
    }),
  ]

  depends_on = [helm_release.kube_prometheus_stack]
}

resource "helm_release" "alloy_gateway" {
  name       = "alloy-gateway"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "alloy"
  version    = var.alloy_version
  namespace  = kubernetes_namespace.this.metadata[0].name
  timeout    = 300
  wait       = false

  values = [file("${path.module}/helm-values/alloy-gateway.yaml")]

  depends_on = [helm_release.tempo]
}
