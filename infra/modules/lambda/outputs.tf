output "functions" {
  description = "Created functions keyed by short name: name, arn and invoke_arn"
  value = {
    for key, fn in aws_lambda_function.this : key => {
      name       = fn.function_name
      arn        = fn.arn
      invoke_arn = fn.invoke_arn
    }
  }
}
