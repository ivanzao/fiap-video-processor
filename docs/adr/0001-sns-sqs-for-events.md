# SNS e SQS como barramento de eventos

**Status:** aceito

A api e o worker trocam eventos assíncronos (`VideoProcessingRequested` e os desfechos). Escolhemos
SNS com um tópico por publicador e SQS com uma fila por consumidor, em vez de RabbitMQ ou Kafka
em cluster, porque são gerenciados, custam quase nada em volume de projeto acadêmico e não exigem
operar brokers no EKS. A api publica via transactional outbox para não perder eventos entre a
transação e o SNS; o worker não precisa de outbox porque só apaga a mensagem da fila depois de
publicar o desfecho, e uma entrega duplicada é absorvida pela tabela `processing_execution`.

## Opções consideradas

- **Kafka (MSK)**: reprocessamento por offset seria útil, mas o custo fixo e a operação não se
  justificam para dois consumidores.
- **RabbitMQ no cluster**: mais um componente com estado para operar dentro do EKS.
- **SQS direto, sem SNS**: perde o fan-out; com o SNS um novo consumidor é só mais uma assinatura.

## Consequências

- Ordem entre eventos não é garantida; a api só aceita transições de Status permitidas e ignora
  o resto.
- O escalonamento do worker fica trivial com o KEDA observando o tamanho da fila.
- Mensagens que estouram o `maxReceiveCount` caem na DLQ e o redrive é manual.
