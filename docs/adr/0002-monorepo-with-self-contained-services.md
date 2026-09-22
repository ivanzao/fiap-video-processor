# Monorepo com serviços autocontidos e sem bibliotecas compartilhadas

**Status:** aceito

Os três deployables vivem num único repositório, cada um com seu `go.mod`, seu Dockerfile, suas
migrações e seus manifestos Kubernetes, e nenhum importa código dos outros. O contrato de eventos
e os headers de identidade são duplicados em `internal/` de cada serviço.

O motivo é o tamanho do problema. O sistema tem três deployables, um contrato de quatro eventos,
um agregado por serviço e uma pessoa desenvolvendo. Um repositório por serviço custa pipelines,
históricos e um repositório de infraestrutura separado para coordenar contratos, e esse custo só
se paga quando há times ou ciclos de release independentes. A proibição de código compartilhado
garante que qualquer serviço pode ser movido para um repositório próprio sem desemaranhar
dependências: a estrutura interna de cada um é a mesma que ele teria sozinho.

## Alternativa rejeitada

Um repositório por serviço mais um de infraestrutura. Descartado pelo tamanho da solução, não por
princípio; a regra de zero código compartilhado existe exatamente para que a extração continue
possível se o projeto crescer.

## Consequências

- Uma mudança no envelope de eventos é feita duas vezes, o que é aceitável para um envelope de
  cinco campos e é o preço de não criar uma biblioteca versionada.
- Os workflows de CI filtram por caminho, então tocar um serviço não roda o pipeline dos outros.
- Cada serviço tem um README próprio em `services/<svc>/README.md`, que é o que sobreviveria a
  uma extração.
