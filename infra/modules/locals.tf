locals {
  name         = var.name
  namespace    = "${local.name}-${var.environment}"
  cluster_name = "${local.name}-${var.environment}-cluster"
  account_id   = data.aws_caller_identity.current.account_id
  lab_role_arn = "arn:aws:iam::${local.account_id}:role/LabRole"
  db_prefix    = replace(local.name, "-", "_")

  services = {
    api    = { http = true, database = true, events = true, subscribes_to = ["worker"], visibility_timeout_seconds = 30 }
    worker = { http = false, database = true, events = true, subscribes_to = ["api"], visibility_timeout_seconds = 300 }
    auth   = { http = false, database = true, events = false, subscribes_to = [], visibility_timeout_seconds = 30 }
  }

  http_services = {
    for idx, name in sort([for name, svc in local.services : name if svc.http]) : name => {
      node_port     = 30080 + idx
      listener_port = 8080 + idx
    }
  }

  databases = {
    for name, svc in local.services : name => {
      db_name = "${local.db_prefix}_${name}"
      role    = "app_${name}"
    } if svc.database
  }

  messaging_services = {
    for name, svc in local.services : name => {
      subscribes_to              = svc.subscribes_to
      visibility_timeout_seconds = svc.visibility_timeout_seconds
    } if svc.events
  }

  identity_headers = {
    "X-User-Id"    = "$context.authorizer.userId"
    "X-User-Email" = "$context.authorizer.email"
  }
}
