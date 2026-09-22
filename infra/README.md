# Infraestrutura

Terraform de cada ambiente do FIAP X Video Processor. O módulo-raiz em `modules/` (`main.tf`,
`locals.tf`, `ssm.tf`) compõe os módulos-folha em `modules/<nome>/`, e as raízes finas
`environments/{staging,prod}` só passam variáveis (CIDR, tamanho dos nós, secrets). O registro
de serviços fica em `modules/locals.tf`: adicionar um serviço é uma entrada no mapa.

## O que cada ambiente cria

| Camada | Recursos |
|---|---|
| Plataforma | VPC própria, EKS 1.34 (`t3.medium`, 3 nós em cada ambiente; um só não comporta os add-ons, que ocupam os 17 slots de pods de um `t3.medium`), ECR pull-through para o GHCR, External Secrets, KEDA, observabilidade (kube-prometheus-stack, Tempo, Alloy, dashboards e alertas) |
| Aplicação | namespace, uma instância RDS PostgreSQL por serviço com credenciais no Secrets Manager, bucket S3 com CORS e lifecycle de 7 dias em `uploads/`, tópicos SNS e filas SQS com DLQ, três Lambdas, API Gateway com authorizer, VPC Link e NLB, parâmetros SSM |

Global na conta: o bucket de state (uma chave por ambiente) e o repositório ECR da imagem
placeholder das Lambdas, que o pipeline cria e semeia quando não existem.

Toda identidade AWS é a `LabRole` via IMDS ([ADR 0004](../docs/adr/0004-labrole-via-imds.md)).
No laboratório o pipeline também cria os buckets de Videos e de traces antes do apply, porque a
política do AWS Academy impede o provider de lê-los; em outra conta basta `manage_buckets = true`
e o Terraform os cria.

O primeiro apply de um ambiente é feito em dois passes (`vpc`, `eks` e `registry`, depois o
restante), porque os providers Kubernetes e Helm dependem do cluster criado no mesmo state. O
pipeline cuida disso e o PR check confere que cada target da lista ainda existe em `modules/main.tf`.

## Roteamento

O API Gateway HTTP API de cada ambiente é o único ponto de entrada público:

| Rota | Destino | Authorizer |
|---|---|---|
| `POST /auth/v1/signup`, `POST /auth/v1/login` | Lambdas `signup` e `login` | não |
| `ANY /v1/{proxy+}` | api, por VPC Link e NLB | sim, injeta `X-User-Id` e `X-User-Email` |
| `GET /`, `GET /{proxy+}`, `GET /health` | api (página web e assets) | não, remove os headers de identidade |

O NLB aponta para o NodePort da api, publicado no SSM por ambiente; o worker não tem rota pública.

## Contrato de Integração (SSM Parameter Store)

O deploy dos serviços consome os outputs da infra por SSM, sem `terraform_remote_state`.

| Parâmetro (`/video-processor/<env>/...`) | Publicado por | Conteúdo |
|---|---|---|
| `eks/cluster-name`, `namespace` | `modules/ssm.tf` | destino do `kubectl` |
| `s3/bucket` | `modules/storage` | `S3_BUCKET` |
| `sns/<svc>-events-topic-arn`, `sqs/<svc>-inbox-url`, `sqs/<svc>-inbox-name` | `modules/messaging` | `SNS_TOPIC_ARN`, `SQS_QUEUE_URL` e a fila do ScaledObject |
| `<svc>/node-port` | `modules/ssm.tf` | NodePort do Service da api, fonte única para o overlay |
| `<svc>/db/secret-arn` | `modules/rds` | ARN do secret com `host, port, dbname, username, password, url` da instância do serviço, lido pelo External Secrets |
| `worker/mailersend-secret-arn` | `modules/ssm.tf` | token e remetente do MailerSend |
| `eso/cluster-secret-store` | `modules/external-secrets` | nome do ClusterSecretStore referenciado pelos `ExternalSecret` |
| `otel/exporter-otlp-endpoint` | `modules/observability` | destino dos traces |
| `lambda/<fn>-function-name` | `modules/lambda` | destino do `update-function-code` |
| `apigw/endpoint` | `modules/gateway` | URL pública, usada no smoke test pós-deploy |

## Secrets do pipeline

| Secret | Uso |
|---|---|
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` | credenciais do AWS Academy (rotacionam a cada sessão) |
| `GHCR_TOKEN` | PAT `read:packages` para o ECR pull-through e o pull secret do cluster |
| `MAILERSEND_TOKEN` | token do MailerSend usado pelo worker |
| `MAIL_FROM` (variable, não secret) | remetente das Notifications; no trial do MailerSend é o endereço do domínio de teste atrelado ao token, então os dois são trocados juntos |
| `SONAR_TOKEN` | token do SonarCloud |

GitHub Environments: `staging` sem aprovador e `production` com aprovador obrigatório. Tudo o mais
(endpoints, ARNs, credenciais dos bancos, que o Terraform gera) sai do SSM e do Secrets Manager.

Para trocar o token do MailerSend: atualize `MAILERSEND_TOKEN` e `MAIL_FROM` no GitHub, rode o
`infra deploy` (o Terraform regrava o secret `video-processor/<env>/worker/mailersend`) e reinicie o
worker (`kubectl rollout restart deployment/video-processor-worker -n video-processor-<env>`), porque
o External Secrets atualiza o Secret do cluster mas os pods só releem variáveis de ambiente ao subir.

## Acesso Local aos Serviços

O Grafana e os RDS ficam dentro da VPC de cada ambiente. Os scripts abaixo abrem túneis:

```bash
# Grafana em http://localhost:3000; imprime a senha do admin e os links dos dashboards
./scripts/grafana-tunnel.sh staging

# RDS: um túnel por serviço, cada um em porta própria
./scripts/rds-tunnel.sh api    staging   # localhost:15432
./scripts/rds-tunnel.sh worker staging   # localhost:15433
./scripts/rds-tunnel.sh auth   staging   # localhost:15434
```

Pré-requisitos: `aws cli` com credenciais válidas, `kubectl`, `jq` (só no túnel de RDS).

Os dashboards do Grafana vivem em `modules/observability/dashboards/` e são validados por
`make validate-dashboards`. O Alertmanager fica desligado: não há canal de notificação configurado
e cada pod conta no orçamento de pods de um `t3.medium`; os alertas ficam visíveis no Grafana.
