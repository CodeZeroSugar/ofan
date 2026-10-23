#!/usr/bin/env bash
set -euo pipefail

cat <<'BANNER'
   ____  _____    _    _   _
  / __ \|  ___|  / \  | \ | |
 | |  | | |_    / _ \ |  \| |
 | |__| |  _|  / ___ \| |\  |
  \____/|_|   /_/   \_\_| \_|

      ⚔  Valheim server manager  ⚔
BANNER

cd "$(dirname "$0")"
export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"

FRESH_INSTALL=no

# check prerequisites
command -v openssl >/dev/null || { echo "openssl not found"; exit 1;}
command -v kubectl >/dev/null || { echo "kubectl not found"; exit 1;}
kubectl get nodes >/dev/null

# apply manifests
kubectl apply -f infra/00-namespace.yaml
kubectl apply -f infra/10-rbac.yaml

if ! kubectl -n ofan get pvc ofan-data >/dev/null 2>&1; then
  FRESH_INSTALL=yes
fi
kubectl apply -f infra/30-pvc.yaml

# create the secret if missing
if ! kubectl -n ofan get secret ofan-app >/dev/null 2>&1; then
  if [[ "$FRESH_INSTALL" = "no" ]]; then
    echo "WARNING: secret does not exist on this install, database keeps its original password. Logging in needs the old password or a database reset."
  fi
  kubectl -n ofan create secret generic ofan-app \
    --from-literal=OFAN_SESSION_SECRET="$(openssl rand -hex 32)" \
    --from-literal=OFAN_ROOT_PASS="$(openssl rand -hex 16)"
fi

kubectl apply -f infra/40-deployment.yaml
kubectl apply -f infra/50-service.yaml

# wait for app to start
if ! kubectl -n ofan rollout status deploy/ofan --timeout=180s >/dev/null 2>&1; then
  kubectl -n ofan logs deploy/ofan --tail=30
  exit 1
fi

echo "=============================================="
echo "  Your dashboard is ready. ᚠᛅᚱ ᚼᛅᛁᛚ᛬          "
echo "=============================================="

# print URL and password location
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
echo "Open: http://${NODE_IP}:30080"

if [[ "$FRESH_INSTALL" = "yes" ]]; then
  kubectl -n ofan get secret ofan-app -o jsonpath='{.data.OFAN_ROOT_PASS}' | base64 -d; echo
else
  echo "existing password is unchanged, can be changed in web dashboard"
fi

