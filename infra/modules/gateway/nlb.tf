resource "aws_security_group" "this" {
  name        = "${local.nlb_name}-nlb-sg"
  description = "Security group for the private app NLB"
  vpc_id      = var.vpc_id

  tags = { Name = "${local.nlb_name}-nlb-sg" }
}

resource "aws_lb" "this" {
  name               = local.nlb_name
  internal           = true
  load_balancer_type = "network"
  security_groups    = [aws_security_group.this.id]
  subnets            = var.private_subnet_ids

  tags = { Name = local.nlb_name }
}

resource "aws_lb_target_group" "service" {
  for_each = var.http_services

  name                 = "${var.name}-${each.key}-${var.environment}"
  port                 = each.value.node_port
  protocol             = "TCP"
  target_type          = "instance"
  vpc_id               = var.vpc_id
  deregistration_delay = 30
  preserve_client_ip   = "false"

  health_check {
    enabled             = true
    healthy_threshold   = 3
    interval            = 30
    port                = "traffic-port"
    protocol            = "HTTP"
    path                = "/health"
    unhealthy_threshold = 3
  }

  tags = { Name = "${var.name}-${each.key}-${var.environment}" }
}

resource "aws_autoscaling_attachment" "service" {
  for_each = local.service_asg_attachments

  autoscaling_group_name = each.value.asg
  lb_target_group_arn    = aws_lb_target_group.service[each.value.service].arn
}

resource "aws_lb_listener" "service" {
  for_each = var.http_services

  load_balancer_arn = aws_lb.this.arn
  port              = each.value.listener_port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.service[each.key].arn
  }
}

resource "aws_security_group_rule" "vpc_link_to_listener" {
  for_each = var.http_services

  type                     = "ingress"
  security_group_id        = aws_security_group.this.id
  source_security_group_id = var.vpc_link_sg_id
  from_port                = each.value.listener_port
  to_port                  = each.value.listener_port
  protocol                 = "tcp"
  description              = "API Gateway VPC Link to the ${each.key} listener"
}

resource "aws_security_group_rule" "nlb_to_nodes" {
  for_each = var.http_services

  type                     = "egress"
  security_group_id        = aws_security_group.this.id
  source_security_group_id = var.eks_cluster_sg_id
  from_port                = each.value.node_port
  to_port                  = each.value.node_port
  protocol                 = "tcp"
  description              = "NLB to EKS nodes on the ${each.key} NodePort"
}

resource "aws_security_group_rule" "nodes_from_nlb" {
  for_each = var.http_services

  type                     = "ingress"
  security_group_id        = var.eks_cluster_sg_id
  source_security_group_id = aws_security_group.this.id
  from_port                = each.value.node_port
  to_port                  = each.value.node_port
  protocol                 = "tcp"
  description              = "EKS nodes receive ${each.key} NodePort traffic from the NLB"
}
