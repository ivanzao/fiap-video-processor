# video-processor-auth

Cadastro e login por e-mail e senha, emissão do JWT e o authorizer do API Gateway. Roda como três
Lambdas na AWS e como um servidor HTTP no Compose. Visão geral e deploy estão no
[README da raiz](../../README.md) e o porquê do monorepo no [ADR 0002](../../docs/adr/0002-monorepo-with-self-contained-services.md); o diagrama deste serviço está em
[`docs/diagrams/03-service-layers.md`](../../docs/diagrams/03-service-layers.md) e o modelo de
confiança nos headers no [ADR 0003](../../docs/adr/0003-auth-in-lambda-and-header-trust.md).

## Fronteiras

```
internal/
├── domain/user/              # User, Credentials, Session, Identity, ports
│                             #   Service.SignUp · Service.Login · Authorizer.Authorize
├── adapters/
│   ├── inbound/httpapi/      # POST /auth/v1/signup e /login, GET /health (dto, errors, router, handler)
│   ├── inbound/lambdahttp/   # converte o evento do API Gateway v2 em net/http e volta
│   ├── inbound/authorizer/   # handler do Lambda authorizer: lê o Bearer, devolve o contexto
│   ├── outbound/postgres/    # UserRepository (tabela user)
│   ├── outbound/password/    # bcrypt
│   └── outbound/jwt/         # HS256: emite e verifica
├── platform/                 # config, observability, system
├── app/                      # SignUpLambda, LoginLambda, AuthorizerLambda, RunLocalServer, Migrate
└── testinfra/                # testcontainers (Postgres)
```

O mesmo `http.Handler` do `httpapi` atende as Lambdas (através do `lambdahttp`) e o servidor
local, então o Compose exercita o mesmo código que a AWS. Regras de importação:
[ADR 0005](../../docs/adr/0005-hexagonal-layout-with-import-rules.md).

## Binários

| Binário | Onde roda | O que faz |
|---|---|---|
| `cmd/signup` | Lambda `signup` (imagem `target: lambda`, `LAMBDA_NAME=signup`) | `POST /auth/v1/signup` |
| `cmd/login` | Lambda `login` | `POST /auth/v1/login`, devolve o JWT com `sub` e `email` |
| `cmd/authorizer` | Lambda authorizer do API Gateway (`ANY /v1/{proxy+}`) | valida o Bearer e devolve `userId` e `email`, que o gateway injeta como `X-User-Id` e `X-User-Email` |
| `cmd/migrate` | Job no EKS (`target: migrate`), `migrate-auth` no Compose | aplica `db/migrations/*.up.sql` |
| `cmd/local` | só no Compose (`target: local`, serviço `auth`) | servidor HTTP na 8081 com signup, login e health, no lugar das Lambdas |

O Dockerfile tem um alvo por papel; a CI publica `signup`, `login`, `authorizer` (arm64) e
`migrate`.

## Variáveis de ambiente

| Variável | Obrigatória | Default | Uso |
|---|---|---|---|
| `DATABASE_URL` | signup, login, local, migrate | | DSN do banco `video_processor_auth` |
| `JWT_SECRET` | todos exceto migrate | | segredo HS256; o authorizer só precisa dele |
| `TOKEN_TTL` | não | `1h` | validade do JWT |
| `BCRYPT_COST` | não | `10` | custo do hash |
| `PORT` | não | `8081` | porta do `cmd/local` |

## Execução e testes

```bash
go test ./...                         # unitários: domínio, handlers, lambdahttp, authorizer, jwt, bcrypt, config
go test -tags integration ./...       # + repositório e migrações (Docker)
golangci-lint run ./...
docker compose up auth                # na raiz do repositório
```
