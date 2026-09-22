output "invoke_url" {
  description = "Public base URL of the HTTP API"
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "api_id" {
  description = "HTTP API ID"
  value       = aws_apigatewayv2_api.this.id
}

output "nlb_arn" {
  description = "Private app NLB ARN"
  value       = aws_lb.this.arn
}
