# video-processor-api

Serviço HTTP que cria o Video, assina a URL de upload, confirma o envio criando a Process Request,
acompanha o Status pelos eventos do worker e emite o link de download do Process Result. Também
serve a página web. Roda no EKS atrás do API Gateway. Visão geral, contratos e deploy estão no
[README da raiz](../../README.md) e o porquê do monorepo no [ADR 0002](../../docs/adr/0002-monorepo-with-self-contained-services.md); o diagrama deste serviço está em
[`docs/diagrams/03-service-layers.md`](../../docs/diagrams/03-service-layers.md).

## Fronteiras

```
internal/
├── domain/video/           # Video, ProcessRequest, ProcessResult, Status, ports e o Service
│                           #   CreateVideo · ProcessVideo · ListVideos · DownloadResult
│                           #   StartProcessing · CompleteProcessing · FailProcessing
├── adapters/
│   ├── inbound/httpapi/    # rotas /v1/videos, RequireIdentity, DTOs e mapa de erros
│   ├── inbound/sqs/        # consome os eventos do worker e chama os casos de uso
│   ├── outbound/postgres/  # VideoRepository e Outbox (pgx, SQL manual)
│   ├── outbound/s3/        # presign de upload com metadado de dono, HEAD, presign de download
│   ├── outbound/outbox/    # relay que publica o outbox no SNS
│   └── outbound/sns/       # Publisher do envelope de eventos
├── platform/               # config, observability, metrics, system (Clock, UUID), events (contrato)
├── app/                    # raiz de composição: liga cada port ao seu adapter
├── testinfra/              # testcontainers (Postgres, LocalStack) para os testes de integração
└── e2e/                    # teste ponta a ponta do serviço contra os containers
```

O domínio não importa nada de `internal/`; adapters importam domínio e platform, nunca outro
adapter; `cmd/api` só importa `app` e `platform`. As regras estão no `.golangci.yml` da
raiz e no [ADR 0005](../../docs/adr/0005-hexagonal-layout-with-import-rules.md).

## Binários

| Binário | Onde roda | O que faz |
|---|---|---|
| `cmd/api` | Deployment no EKS (`infra/k8s/`), `api` no Compose | servidor HTTP na porta 8080, `/metrics` e `/health` na 9090, relay do outbox e consumer SQS em goroutines |
| `cmd/api migrate` | Job no EKS antes do rollout, `migrate-api` no Compose | aplica `db/migrations/*.up.sql` com golang-migrate |

## Variáveis de ambiente

| Variável | Obrigatória | Default | Uso |
|---|---|---|---|
| `DATABASE_URL` | sim | | DSN do banco `video_processor_api` |
| `S3_BUCKET` | sim | | bucket dos Videos e dos zips |
| `SNS_TOPIC_ARN` | sim | | tópico `api-events` onde o outbox publica |
| `SQS_QUEUE_URL` | sim | | fila `api-inbox` com os eventos do worker |
| `AWS_REGION` | não | `us-east-1` | região dos clientes AWS |
| `AWS_ENDPOINT_URL` | não | | endpoint alternativo (LocalStack) |
| `S3_PUBLIC_ENDPOINT` | não | | endpoint usado só para assinar URLs que o browser acessa (LocalStack visto do host) |
| `PORT` | não | `8080` | porta HTTP |
| `METRICS_PORT` | não | `9090` | porta de `/metrics` |
| `MAX_UPLOAD_BYTES` | não | `524288000` | limite de tamanho do Video (500 MiB) |
| `UPLOAD_TTL` | não | `15m` | validade da URL de upload |
| `DOWNLOAD_TTL` | não | `1h` | validade da URL de download |
| `OUTBOX_INTERVAL` | não | `2s` | intervalo do relay |
| `OUTBOX_BATCH` | não | `50` | eventos por rodada do relay |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | não | | destino dos traces; sem ele o tracing fica desligado |

## Execução e testes

```bash
go test ./...                         # unitários: domínio, handlers, consumer, métricas
go test -tags integration ./...       # + repositório, migrações e e2e do serviço (Docker)
golangci-lint run ./...               # usa o .golangci.yml da raiz
docker compose up api                 # na raiz do repositório, com as dependências
```

Os testes do domínio usam fakes escritos à mão (`domain/video/fakes_test.go`); os de integração
sobem Postgres e LocalStack com testcontainers.
