# video-processor-worker

Consumidor de `VideoProcessingRequested` que executa a Process Request: baixa o Video, extrai um
frame por segundo com ffmpeg, empacota o zip, publica o desfecho e envia a Notification por
e-mail. Roda no EKS escalado pelo KEDA. Visão geral, contratos e deploy estão no
[README da raiz](../../README.md) e o porquê do monorepo no [ADR 0002](../../docs/adr/0002-monorepo-with-self-contained-services.md); o diagrama deste serviço está em
[`docs/diagrams/03-service-layers.md`](../../docs/diagrams/03-service-layers.md).

## Fronteiras

```
internal/
├── domain/video/            # ProcessRequest, Execution (Attempt, Outcome), eventos, Notification
│                            #   Service.ExecuteProcessRequest e os sete ports
├── adapters/
│   ├── inbound/sqs/         # consumer com concorrência, heartbeat de visibilidade e devolução à fila
│   ├── outbound/postgres/   # ExecutionRepository: processing_execution e notification_log
│   ├── outbound/s3/         # GET do Video e PUT do zip
│   ├── outbound/ffmpeg/     # extração de frames; separa defeito do arquivo de falha transitória
│   ├── outbound/zip/        # empacotamento dos frames
│   ├── outbound/workspace/  # diretório temporário por Execution
│   ├── outbound/sns/        # Publisher dos três eventos de desfecho
│   └── outbound/mail/       # MailerSend (AWS) e SMTP (Mailpit local)
├── platform/                # config, observability, metrics, system, events
├── app/                     # raiz de composição
├── testinfra/               # testcontainers (Postgres, LocalStack, Mailpit)
└── e2e/                     # teste ponta a ponta do serviço com um vídeo sintético
```

Regras de importação entre as camadas: [ADR 0005](../../docs/adr/0005-hexagonal-layout-with-import-rules.md).

## Binários

| Binário | Onde roda | O que faz |
|---|---|---|
| `cmd/worker` | Deployment no EKS com ScaledObject do KEDA (1 a 6 réplicas), `worker` no Compose | consome a `worker-inbox`; `/health` e `/metrics` na 9090 |
| `cmd/worker migrate` | Job no EKS antes do rollout, `migrate-worker` no Compose | aplica `db/migrations/*.up.sql` |

A imagem inclui o ffmpeg. Cada mensagem é uma Execution: o worker publica `VideoProcessingStarted`,
processa, publica `Completed` ou `Failed`, envia a Notification e só então apaga a mensagem.
Qualquer falha antes do ack devolve a mensagem à fila; a próxima entrega encontra a Execution
terminal e só republica. Detalhes em
[`docs/diagrams/02-failure-scenarios.md`](../../docs/diagrams/02-failure-scenarios.md).

## Variáveis de ambiente

| Variável | Obrigatória | Default | Uso |
|---|---|---|---|
| `DATABASE_URL` | sim | | DSN do banco `video_processor_worker` |
| `S3_BUCKET` | sim | | bucket dos Videos e dos zips |
| `SNS_TOPIC_ARN` | sim | | tópico `worker-events` |
| `SQS_QUEUE_URL` | sim | | fila `worker-inbox` |
| `SMTP_ADDR` ou `MAILERSEND_TOKEN` | um dos dois | | SMTP local (Mailpit) ou token do MailerSend |
| `MAIL_FROM` | não | `noreply@fiapx.example` | remetente das Notifications |
| `MAILERSEND_ENDPOINT` | não | `https://api.mailersend.com` | API do MailerSend |
| `AWS_REGION` | não | `us-east-1` | região dos clientes AWS |
| `AWS_ENDPOINT_URL` | não | | endpoint alternativo (LocalStack) |
| `WORKER_CONCURRENCY` | não | `2` | Executions simultâneas por pod |
| `SQS_VISIBILITY_TIMEOUT` | não | `5m` | visibilidade da mensagem em processamento |
| `SQS_HEARTBEAT` | não | `1m` | renovação da visibilidade enquanto o ffmpeg roda |
| `SQS_RETRY_DELAY` | não | `30s` | quanto a mensagem espera antes de voltar após uma Retryable Failure |
| `FRAME_RATE` | não | `1` | frames por segundo extraídos (Frame Rate da plataforma) |
| `MAX_ATTEMPTS` | não | `3` | Attempts antes de `FAILED` |
| `FFMPEG_BINARY` | não | `ffmpeg` | caminho do binário |
| `FFMPEG_THREADS` | não | `1` | threads do ffmpeg por Execution |
| `WORK_DIR` | não | diretório temporário do sistema | raiz das áreas de trabalho |
| `METRICS_PORT` | não | `9090` | porta de `/health` e `/metrics` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | não | | destino dos traces |

## Execução e testes

```bash
go test ./...                         # unitários: domínio, ffmpeg (pula sem o binário no PATH), zip, métricas
go test -tags integration ./...       # + repositório, e-mail e e2e do serviço (Docker e ffmpeg)
golangci-lint run ./...
docker compose up worker              # na raiz do repositório
```
