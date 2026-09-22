locals {
  ssm_prefix = "/${local.name}/${var.environment}"

  ssm_parameters = merge(
    {
      "eks/cluster-name"             = module.eks.cluster_name
      "namespace"                    = kubernetes_namespace.this.metadata[0].name
      "s3/bucket"                    = module.storage.bucket_name
      "apigw/endpoint"               = module.gateway.invoke_url
      "eso/cluster-secret-store"     = module.external_secrets.cluster_secret_store_name
      "otel/exporter-otlp-endpoint"  = module.observability.otlp_http_endpoint
      "worker/mailersend-secret-arn" = aws_secretsmanager_secret.mailersend.arn
    },
    { for name, instance in module.rds : "${name}/db/secret-arn" => instance.secret_arn },
    { for name, ports in local.http_services : "${name}/node-port" => tostring(ports.node_port) },
    { for name, arn in module.messaging.topic_arns : "sns/${name}-events-topic-arn" => arn },
    { for name, url in module.messaging.queue_urls : "sqs/${name}-inbox-url" => url },
    { for name, queue in module.messaging.queue_names : "sqs/${name}-inbox-name" => queue },
    { for name, fn in module.lambda.functions : "lambda/${name}-function-name" => fn.name },
  )
}

resource "aws_ssm_parameter" "this" {
  for_each = local.ssm_parameters

  name  = "${local.ssm_prefix}/${each.key}"
  type  = "String"
  value = each.value
}
