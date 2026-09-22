# Isolamento por ambiente e por serviço

**Status:** aceito

Cada ambiente (`staging`, `prod`) é um stack completo e independente: VPC própria, cluster EKS
próprio com External Secrets, KEDA e observabilidade, bucket, filas, Lambdas e API Gateway. Dentro
do ambiente, cada serviço tem sua própria instância RDS PostgreSQL
(`video-processor-<svc>-<env>-db`, seis `db.t3.micro` no total); o usuário master da instância é o
próprio usuário do serviço, com senha gerada pelo Terraform e entregue pelo Secrets Manager e pelo
External Secrets. Localmente o Compose sobe um container Postgres por serviço, em portas distintas.

O Terraform tem um único módulo-raiz em `infra/modules/`, composto por módulos-folha, e uma raiz
fina por ambiente em `infra/environments/<env>/` que só passa variáveis. O que continua global na
conta é o que dois states não podem possuir ao mesmo tempo: o bucket de state (uma chave por
ambiente) e o repositório ECR da imagem placeholder das Lambdas, que o pipeline cria e semeia de
forma idempotente antes do apply.

## Alternativas rejeitadas

- **Um cluster compartilhado com um namespace por ambiente.** Economiza nós, mas exige uma
  terceira raiz de Terraform para o que é comum, outputs cruzados entre states e nomes singleton,
  e staging e produção no mesmo cluster não são ambientes separados de verdade.
- **Uma instância RDS com um banco por serviço.** O isolamento lógico seria real, mas a fronteira
  entre os serviços pararia no `CREATE DATABASE`: um problema de capacidade, uma manutenção ou um
  `DROP` errado na instância atingiria os três. Com uma instância por serviço, cada um pode ser
  dimensionado, migrado ou extraído para um repositório próprio sem tocar nos outros, e o desenho
  local espelha o da nuvem.

## Consequências

- Dois clusters, dois NLB, seis RDS e duas pilhas de observabilidade quando os dois ambientes
  estão de pé; na demonstração só o `staging` é usado, mas a separação existe no código e no
  pipeline.
- O primeiro apply de cada ambiente é feito em dois passes (`vpc`, `eks`, `registry`, depois o
  restante), porque os providers Kubernetes e Helm dependem do cluster criado no mesmo state; o
  PR check confere que cada target da lista ainda existe em `infra/modules/main.tf`.
- Não existe senha master compartilhada nem Job in-cluster criando bancos e roles.
- NodePorts fixos (`30080 + índice`) em cada cluster; parâmetros SSM só em
  `/video-processor/<env>/...`; os túneis de Grafana e RDS recebem o ambiente.
