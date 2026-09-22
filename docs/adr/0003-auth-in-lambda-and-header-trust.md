# Autenticação em Lambda e confiança nos headers de identidade

**Status:** aceito

Cadastro, login e validação do JWT rodam em Lambdas atrás do API Gateway. O authorizer devolve
`userId` e `email` e o gateway os injeta como `X-User-Id` e `X-User-Email` com `overwrite:`, o que
impede o cliente de forjá-los; a api lê os headers e confia, sem conhecer o segredo do JWT. Assim a
api não precisa do segredo nem revalida assinaturas, e o resultado do authorizer é cacheado pelo
gateway por 5 minutos.

## Alternativa rejeitada

Validar o JWT dentro da api. Exigiria distribuir o segredo para cada serviço que expõe rotas e
repetir a validação em cada um; com o authorizer, a regra vive num único lugar e um serviço novo
atrás do gateway já nasce protegido.

## Consequências

- Localmente não há authorizer: o gateway nginx repassa os headers que a página web decodifica
  do próprio JWT. É conveniente e inseguro por construção, e só existe no Compose.
- Rotas públicas (`/`, `/health`, assets) removem os headers de identidade antes de encaminhar.
- Trocar o formato das claims exige redeploy do authorizer e da configuração do gateway.
