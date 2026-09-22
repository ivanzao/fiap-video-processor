# LabRole via IMDS como única identidade AWS

**Status:** aceito

O AWS Academy não permite criar roles ou policies IAM, o que inviabiliza IRSA e roles por workload.
Todos os componentes que falam com a AWS (pods da api e do worker, KEDA, External Secrets, as
Lambdas e o próprio EKS) usam a `LabRole` existente. Nos nós do EKS ela chega pelo IMDS com
`http_put_response_hop_limit = 2` no launch template, para que os pods a alcancem.

A mesma organização aplica uma service control policy que nega `s3:GetBucketObjectLockConfiguration`
em qualquer bucket. O provider AWS do Terraform lê essa configuração ao criar ou atualizar um
`aws_s3_bucket` e não tolera o `AccessDenied` (issue aberta do provider,
[#7550](https://github.com/hashicorp/terraform-provider-aws/issues/7550)), então um apply que cria
bucket falha. Os módulos `storage` e `observability` continuam declarando o `aws_s3_bucket` (a
prática normal em qualquer outra conta), mas os ambientes do laboratório aplicam com
`manage_buckets = false`: o bucket é criado pelo passo de bootstrap do pipeline, como o bucket de
state, e o Terraform gerencia só CORS, lifecycle, criptografia e bloqueio de acesso público.

## Consequências

- Sem isolamento de identidade entre workloads; aceitável num cluster de laboratório single-tenant.
- Em produção real: roles dedicadas com privilégio mínimo por workload, via IRSA ou Pod Identity.
- Os buckets de um ambiente do laboratório sobrevivem ao `terraform destroy`; para apagá-los é
  `aws s3 rb --force` no bucket de Videos e no de traces.
