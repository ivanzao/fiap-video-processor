#!/usr/bin/env bash
# Abre um port-forward para o Grafana do cluster de um ambiente e imprime a senha do admin.
# Uso: ./scripts/grafana-tunnel.sh [staging|prod]   (Grafana em http://localhost:3000, Ctrl+C encerra)
# Pré-requisitos: aws cli com credenciais válidas, kubectl.

set -euo pipefail

ENV="${1:-staging}"
LOCAL_PORT="${LOCAL_PORT:-3000}"
REGION="${AWS_DEFAULT_REGION:-us-east-1}"
if [[ "$ENV" != "staging" && "$ENV" != "prod" ]]; then
  echo "uso: $0 [staging|prod]" >&2; exit 1
fi

CLUSTER_NAME=$(aws ssm get-parameter --name "/video-processor/$ENV/eks/cluster-name" --query Parameter.Value --output text)
echo "→ atualizando kubeconfig para $CLUSTER_NAME..."
aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$REGION" >/dev/null

ADMIN_PASS=$(kubectl get secret -n observability kube-prometheus-stack-grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d || echo "(veja o secret kube-prometheus-stack-grafana)")

cat <<BANNER

╔═══════════════════════════════════════════════════════════════════
║  Grafana em http://localhost:$LOCAL_PORT
║  login: admin / $ADMIN_PASS
╠═══════════════════════════════════════════════════════════════════
║  Pasta "Video Processor":
║    APM por serviço          /d/video-processor-apm
║    Processing Operations    /d/video-processor-processing
║    Errors                   /d/video-processor-errors
║
║  Explore: /explore
║    sum by (status) (rate(video_process_requests_total[5m]))
║    sum by (outcome) (rate(processing_executions_total[5m]))
║
║  Traces (Tempo): Explore → datasource Tempo → Search por service.name
║  Ctrl+C para encerrar
╚═══════════════════════════════════════════════════════════════════

BANNER

kubectl port-forward -n observability svc/kube-prometheus-stack-grafana "$LOCAL_PORT":80
