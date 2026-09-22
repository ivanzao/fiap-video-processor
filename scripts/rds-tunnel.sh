#!/usr/bin/env bash
# Abre um túnel local para o banco de um serviço via pod socat no cluster.
# Uso: ./scripts/rds-tunnel.sh <api|worker|auth> [staging|prod]
# Portas locais: api 15432 · worker 15433 · auth 15434. Ctrl+C encerra e remove o pod.
# Pré-requisitos: aws cli com credenciais válidas, kubectl, jq.

set -euo pipefail

SERVICE="${1:-}"
ENV="${2:-staging}"
REGION="${AWS_DEFAULT_REGION:-us-east-1}"

case "$SERVICE" in
  api)    LOCAL_PORT=15432 ;;
  worker) LOCAL_PORT=15433 ;;
  auth)   LOCAL_PORT=15434 ;;
  *) echo "uso: $0 <api|worker|auth> [staging|prod]" >&2; exit 1 ;;
esac
if [[ "$ENV" != "staging" && "$ENV" != "prod" ]]; then
  echo "uso: $0 <api|worker|auth> [staging|prod]" >&2; exit 1
fi

NAMESPACE="video-processor-$ENV"
POD_NAME="rds-tunnel-$SERVICE-$$"

SECRET_ARN=$(aws ssm get-parameter --name "/video-processor/$ENV/$SERVICE/db/secret-arn" --query Parameter.Value --output text)
SECRET_JSON=$(aws secretsmanager get-secret-value --secret-id "$SECRET_ARN" --query SecretString --output text)
RDS_HOST=$(echo "$SECRET_JSON" | jq -r .host)
RDS_PORT=$(echo "$SECRET_JSON" | jq -r .port)
DB_NAME=$(echo "$SECRET_JSON" | jq -r .dbname)
DB_USER=$(echo "$SECRET_JSON" | jq -r .username)
DB_PASS=$(echo "$SECRET_JSON" | jq -r .password)

CLUSTER_NAME=$(aws ssm get-parameter --name "/video-processor/$ENV/eks/cluster-name" --query Parameter.Value --output text)
aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$REGION" >/dev/null

cleanup() { kubectl delete pod "$POD_NAME" -n "$NAMESPACE" --ignore-not-found >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

kubectl run "$POD_NAME" --image=alpine/socat --restart=Never --namespace="$NAMESPACE" \
  -- TCP-LISTEN:"$RDS_PORT",fork,reuseaddr TCP:"$RDS_HOST":"$RDS_PORT" >/dev/null
kubectl wait pod "$POD_NAME" --for=condition=Ready --namespace="$NAMESPACE" --timeout=60s >/dev/null

cat <<BANNER

╔═══════════════════════════════════════════════════════════════════
║  Túnel ativo: $SERVICE ($ENV) em localhost:$LOCAL_PORT
║    database: $DB_NAME   user: $DB_USER   password: $DB_PASS
║  psql:
║    PGPASSWORD='$DB_PASS' psql -h localhost -p $LOCAL_PORT -U $DB_USER -d $DB_NAME
║  Ctrl+C encerra e remove o pod
╚═══════════════════════════════════════════════════════════════════

BANNER

kubectl port-forward "pod/$POD_NAME" "$LOCAL_PORT":"$RDS_PORT" --namespace="$NAMESPACE"
