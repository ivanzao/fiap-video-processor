resource "helm_release" "this" {
  name             = "keda"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda"
  version          = var.chart_version
  namespace        = "keda"
  create_namespace = true
  timeout          = 300

  set = [
    { name = "podIdentity.aws.irsa.enabled", value = "false" },
  ]
}
