resource "aws_apigatewayv2_api" "this" {
  name          = "${var.name}-${var.environment}"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.this.id
  name        = "$default"
  auto_deploy = true

  default_route_settings {
    throttling_burst_limit = 100
    throttling_rate_limit  = 50
  }
}

resource "aws_apigatewayv2_vpc_link" "this" {
  name               = "${var.name}-${var.environment}"
  security_group_ids = [var.vpc_link_sg_id]
  subnet_ids         = var.private_subnet_ids
}
