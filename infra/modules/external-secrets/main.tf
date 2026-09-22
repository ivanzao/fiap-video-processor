resource "helm_release" "this" {
  name             = "external-secrets"
  repository       = "https://charts.external-secrets.io"
  chart            = "external-secrets"
  version          = var.chart_version
  namespace        = "external-secrets"
  create_namespace = true
  timeout          = 300
}

resource "helm_release" "resources" {
  name      = "eso-resources"
  chart     = "${path.module}/chart"
  namespace = "external-secrets"
  timeout   = 120

  values = [
    yamlencode({
      storeName  = var.cluster_secret_store_name
      region     = var.aws_region
      namespaces = var.app_namespaces
    })
  ]

  depends_on = [helm_release.this]
}
