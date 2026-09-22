resource "kubernetes_config_map_v1" "dashboard" {
  for_each = fileset("${path.module}/dashboards", "*.json")

  metadata {
    name      = "dashboard-${trimsuffix(each.value, ".json")}"
    namespace = kubernetes_namespace.this.metadata[0].name
    labels = {
      grafana_dashboard = "1"
    }
    annotations = {
      grafana_folder = var.grafana_folder
    }
  }

  data = {
    (each.value) = file("${path.module}/dashboards/${each.value}")
  }

  depends_on = [helm_release.kube_prometheus_stack]
}
