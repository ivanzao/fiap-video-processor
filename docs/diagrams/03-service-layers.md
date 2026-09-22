# 03. Os serviços por dentro

Os três serviços têm a mesma forma: a entrada (HTTP, fila ou evento Lambda) é traduzida em um caso
de uso do domínio, e o domínio sai pelos ports que os adapters de saída implementam. `app` é o
único lugar que sabe qual adapter concreto entra em cada port, e `cmd` só carrega a configuração e
chama `app`. As regras de importação entre as camadas estão no
[ADR 0005](../adr/0005-hexagonal-layout-with-import-rules.md). O desenho abaixo usa a
`video-processor-api`; os nomes são os pacotes reais de `services/video-processor-api/internal/`.

```mermaid
flowchart LR
    classDef inbound fill:#ff9,stroke:#333
    classDef domain fill:#9cf,stroke:#333,stroke-width:2px
    classDef port fill:#fff,stroke:#333,stroke-dasharray: 4 2
    classDef outbound fill:#9f9,stroke:#333
    classDef platform fill:#eee,stroke:#999

    subgraph IN["adapters/inbound"]
        direction TB
        HTTP["httpapi<br/>POST /v1/videos → CreateVideo<br/>POST /v1/videos/id/process → ProcessVideo<br/>GET /v1/videos → ListVideos<br/>GET /v1/videos/id/download → DownloadResult<br/>RequireIdentity lê X-User-Id e X-User-Email"]:::inbound
        SQSIN["sqs<br/>VideoProcessingStarted → StartProcessing<br/>VideoProcessingCompleted → CompleteProcessing<br/>VideoProcessingFailed → FailProcessing"]:::inbound
        WEB["web/ (SPA estática via embed)<br/>GET /"]:::inbound
    end

    subgraph DOM["domain/video"]
        direction TB
        SVC["Service<br/>CreateVideo · ProcessVideo · ListVideos · DownloadResult<br/>StartProcessing · CompleteProcessing · FailProcessing"]:::domain
        AGG["Video · ProcessRequest · ProcessResult<br/>Status e transições permitidas"]:::domain
        P_REPO["port Repository"]:::port
        P_STORE["port ObjectStore"]:::port
        P_CLOCK["port Clock · IDGenerator"]:::port
        P_MET["port Metrics"]:::port
    end

    subgraph OUT["adapters/outbound"]
        direction TB
        PG["postgres<br/>VideoRepository · Outbox"]:::outbound
        S3A["s3<br/>presign PUT com metadado de dono<br/>HEAD · presign GET"]:::outbound
        RELAY["outbox<br/>Relay: lê pendentes, publica, marca"]:::outbound
        SNSA["sns<br/>Publisher do envelope"]:::outbound
    end

    subgraph PLAT["platform"]
        direction TB
        CFG["config"]:::platform
        EVT["events<br/>envelope e payloads"]:::platform
        OBS["observability<br/>slog JSON · OTel"]:::platform
        MET["metrics<br/>Prometheus"]:::platform
        SYS["system<br/>Clock · UUID v7"]:::platform
    end

    APP["app<br/>raiz de composição<br/>cmd/api só carrega config e chama app.Run"]

    HTTP --> SVC
    SQSIN --> SVC
    SVC --- AGG
    SVC --> P_REPO
    SVC --> P_STORE
    SVC --> P_CLOCK
    SVC --> P_MET
    P_REPO -.->|implementa| PG
    P_STORE -.->|implementa| S3A
    P_CLOCK -.->|implementa| SYS
    P_MET -.->|implementa| MET
    PG --> RELAY --> SNSA
    SQSIN --- EVT
    SNSA --- EVT
    PG --- EVT
    APP -.-> IN
    APP -.-> OUT
    APP -.-> PLAT
```

## O mesmo desenho nos outros dois serviços

| | `video-processor-worker` | `video-processor-auth` |
|---|---|---|
| Entrada | `sqs`: consumer com concorrência 2 e heartbeat de visibilidade, `VideoProcessingRequested` → `ExecuteProcessRequest` | `lambdahttp` converte o evento do API Gateway em `net/http` para o `httpapi` (`SignUp`, `Login`); `authorizer` lê o Bearer → `Authorize` |
| Domínio | `domain/video`: `ProcessRequest`, `Execution` (Attempt, Outcome), `Notification`, `UnprocessableError`, `ErrRetryLater` | `domain/user`: `User`, `Credentials`, `Session`, `Identity` |
| Ports | `ObjectStore`, `Extractor`, `Packager`, `Workspace`, `Repository`, `Publisher`, `Notifier`, `Clock`, `IDGenerator`, `Metrics` | `Repository`, `PasswordHasher`, `TokenIssuer`, `TokenVerifier`, `Clock`, `IDGenerator` |
| Saída | `s3`, `ffmpeg` (distingue defeito do arquivo de falha transitória), `zip`, `workspace`, `postgres`, `sns`, `mail` (MailerSend na AWS, SMTP no Compose) | `postgres`, `password` (bcrypt), `jwt` (HS256) |
| Composição | `app.Run` monta o consumer com o `Service` | `SignUpLambda`, `LoginLambda`, `AuthorizerLambda`, `RunLocalServer` (Compose) e `Migrate`, um binário em `cmd/` por papel |

No worker o domínio decide o desfecho da Execution sem saber que existe fila: ele devolve
`ErrRetryLater` quando a mensagem deve voltar, e o consumer traduz isso em visibilidade da mensagem.
No auth o authorizer não abre conexão com o banco: compõe só o verificador JWT com o caso de uso
`Authorize`, e o API Gateway cacheia a resposta por 5 minutos.
