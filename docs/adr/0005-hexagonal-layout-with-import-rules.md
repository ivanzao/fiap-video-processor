# Layout hexagonal por camada com regras de importação verificadas no lint

**Status:** aceito

Cada serviço organiza `internal/` em quatro camadas com o mesmo nome nos três serviços:
`domain/<agregado>` (modelo, casos de uso e ports), `adapters/inbound` (HTTP, fila, evento
Lambda), `adapters/outbound` (Postgres, S3, SNS, ffmpeg, e-mail, JWT, bcrypt), `platform`
(configuração, observabilidade, métricas, relógio, contrato de eventos) e `app` (raiz de
composição). O `main` de cada binário só carrega a configuração e chama `app`. As regras de
importação entre as camadas são verificadas pelo `golangci-lint` na CI:

| Camada | Pode importar |
|---|---|
| `domain` | nada de `internal/`; nem AWS SDK, pgx, `net/http` ou `os/exec` |
| `adapters` | `domain` e `platform`; nunca outro adapter nem `app` |
| `platform` | nada de `adapters` nem `app` |
| `app` | tudo |
| `cmd` | `app` e `platform` (configuração e logger) |

Tipos de bibliotecas externas ficam dentro do adapter que os usa: `postgres.Pool` embrulha o pool
do pgx, e os adapters AWS expõem construtores `Connect` que recebem uma configuração própria, então
`app` não importa pgx nem o SDK. Os casos de uso são nomeados pela instrução que executam
(`CreateVideo`, `ProcessVideo`, `ExecuteProcessRequest`, `Authorize`) e os handlers de cada
adapter de entrada espelham esses nomes.

## Alternativa rejeitada

Uma árvore plana em `internal/`, com agregado, adapters e código transversal lado a lado, e o papel
de cada pacote documentado numa tabela. Documentação envelhece e não falha a CI; a árvore com
camadas nomeadas e as regras no lint mostram a fronteira a quem abre a pasta e impedem que ela se
perca numa refatoração.

## Consequências

- Os testes externos de um adapter (`package x_test`) ficam fora da regra "adapter não importa
  adapter", porque um teste de entrada legitimamente monta um adapter de saída real (o teste do
  authorizer usa o `jwt`). O código de produção dos adapters segue a regra sem exceção.
- Caminhos de importação mais longos (`internal/adapters/outbound/postgres`), mas os nomes de
  pacote continuam curtos (`postgres.NewVideoRepository`).
- Um tipo que dois adapters precisam compartilhar vai para `platform` (o `events.PendingEvent`
  que o `postgres` produz e o `outbox` consome), porque adapter não importa adapter.
- Os três serviços têm a mesma forma (`docs/diagrams/03-service-layers.md`), e um serviço
  extraído para repositório próprio leva a estrutura inteira consigo.
