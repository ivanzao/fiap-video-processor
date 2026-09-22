resource "aws_lambda_function" "this" {
  for_each = var.functions

  function_name = "${var.name}-${each.key}-${var.environment}"
  package_type  = "Image"
  image_uri     = var.image_uri
  role          = var.role_arn
  architectures = ["arm64"]
  timeout       = each.value.timeout
  memory_size   = each.value.memory_size

  vpc_config {
    subnet_ids         = var.private_subnet_ids
    security_group_ids = var.security_group_ids
  }

  dynamic "environment" {
    for_each = length(each.value.environment) > 0 ? [1] : []
    content {
      variables = each.value.environment
    }
  }

  lifecycle {
    ignore_changes = [image_uri]
  }
}
