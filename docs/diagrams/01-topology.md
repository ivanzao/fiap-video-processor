# 01. Topologia

Onde cada componente do FIAP X Video Processor roda na AWS. Cada ambiente (`staging`, `prod`) é
um stack completo e independente: VPC, cluster EKS com seus add-ons, três instâncias RDS (uma por
serviço), bucket, filas, Lambdas e API Gateway próprios. O desenho abaixo é um ambiente.

O upload e o download não passam pelos serviços: a api emite URLs pré-assinadas e o browser fala
direto com o S3 (passos 2 e 3 abaixo). O API Gateway HTTP API limita o corpo a 10 MB, então um
Video de até 500 MB nunca poderia atravessá-lo.

```mermaid
flowchart LR
    classDef external fill:#f9f,stroke:#333,stroke-width:1px
    classDef aws fill:#ff9,stroke:#333,stroke-width:1px
    classDef app fill:#9cf,stroke:#333,stroke-width:1px
    classDef data fill:#9f9,stroke:#333,stroke-width:1px
    classDef obs fill:#fcf,stroke:#333,stroke-width:1px

    User([User no browser]):::external
    MailerSend([MailerSend]):::external

    subgraph APIGW["API Gateway HTTP API"]
        R_auth["POST /auth/v1/signup · login"]
        R_v1["ANY /v1/{proxy+}"]
        R_pub["GET / · /health · assets"]
    end
    class APIGW aws

    subgraph Lambdas["Lambda arm64 · video-processor-auth"]
        L_signup["signup"]:::aws
        L_login["login"]:::aws
        L_auth["authorizer"]:::aws
    end

    subgraph EKS["EKS video-processor-env-cluster · namespace video-processor-env"]
        API["video-processor-api"]:::app
        Worker["video-processor-worker<br/>ffmpeg · KEDA 1..6"]:::app
        ESO["External Secrets"]:::obs
        KEDA["KEDA"]:::obs
    end

    subgraph Data["Dados e mensageria"]
        S3[("S3<br/>uploads/ · results/")]:::data
        RDS_api[("RDS api")]:::data
        RDS_wk[("RDS worker")]:::data
        RDS_auth[("RDS auth")]:::data
        SNS_api["SNS api-events"]:::aws
        SNS_wk["SNS worker-events"]:::aws
        SQS_wk["SQS worker-inbox + DLQ"]:::aws
        SQS_api["SQS api-inbox + DLQ"]:::aws
        Secrets["Secrets Manager"]:::aws
        SSM["SSM Parameter Store"]:::aws
    end

    subgraph Obs["Observabilidade"]
        Alloy["Alloy (OTLP)"]:::obs
        Tempo["Tempo"]:::obs
        Prom["Prometheus"]:::obs
        Grafana["Grafana"]:::obs
    end

    User -->|"e-mail + senha"| R_auth
    User -->|"1. POST /v1/videos com Bearer JWT<br/>api devolve uploadUrl e uploadHeaders"| R_v1
    User --> R_pub
    User -->|"2. PUT direto com a URL pré-assinada<br/>metadado user-id e video-id assinado"| S3
    User -->|"3. GET direto com a URL de download"| S3

    R_auth --> L_signup
    R_auth --> L_login
    R_v1 -->|invoke| L_auth
    L_auth -.->|"X-User-Id · X-User-Email"| R_v1
    R_v1 -->|"VPC Link → NLB"| API
    R_pub -->|"VPC Link → NLB"| API

    L_signup --> RDS_auth
    L_login --> RDS_auth
    API --> RDS_api
    Worker --> RDS_wk
    API -->|"presign · HEAD"| S3
    Worker -->|"GET vídeo · PUT zip"| S3
    API -->|"outbox relay"| SNS_api
    SNS_api --> SQS_wk
    SQS_wk --> Worker
    Worker --> SNS_wk
    SNS_wk --> SQS_api
    SQS_api --> API
    Worker -->|Notification| MailerSend

    KEDA -.->|"queueLength"| SQS_wk
    ESO -.-> Secrets
    API --> Alloy
    Worker --> Alloy
    Alloy --> Tempo
    Alloy --> Prom
    Grafana --> Tempo
    Grafana --> Prom
```

| Cor | Significado |
|---|---|
| Rosa | Atores externos: o User e o provedor de e-mail |
| Amarelo | Serviços gerenciados da AWS |
| Azul | Serviços deste repositório rodando no EKS |
| Verde | Armazenamento de dados |
| Lilás | Operadores e observabilidade |

Localmente o Compose substitui o API Gateway por um nginx que só roteia (`deploy/local/gateway/`),
as Lambdas por `cmd/local` do auth, o S3, SNS e SQS pelo LocalStack e o MailerSend pelo Mailpit.
