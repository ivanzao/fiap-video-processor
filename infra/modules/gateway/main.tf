locals {
  nlb_name = "${var.name}-${var.environment}-app"

  protected_services = toset([for route in values(var.protected_routes) : route.service])
  public_services    = toset([for route in values(var.public_routes) : route.service])
  lambda_functions   = { for key, route in var.lambda_routes : route.function_name => route.invoke_arn }

  service_asg_attachments = {
    for pair in setproduct(keys(var.http_services), var.node_group_asg_names) :
    "${pair[0]}-${pair[1]}" => { service = pair[0], asg = pair[1] }
  }
}
