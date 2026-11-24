#!/usr/bin/env bash
# Deploys Caddy and the proxy to the server with Docker Compose.
#
#   deployment/deploy.sh                         # manual, uses deployment/.env
#   HOST=1.2.3.4 DOMAIN=api.example.com PROXY_IMAGE=ghcr.io/me/proxy:v1 deployment/deploy.sh   # CI
#
# Rollback: run again with the previous PROXY_IMAGE.
set -euo pipefail

cd "$(dirname "$0")"
[ -f .env ] && . ./.env

HOST="${HOST:-$(terraform -chdir=../infrastructure output -raw ip)}"
DOMAIN="${DOMAIN:?set DOMAIN in deployment/.env or the environment}"
PROXY_IMAGE="${PROXY_IMAGE:-traefik/whoami}"
HEALTH_PATH="${HEALTH_PATH:-/}"
DIR=/opt/proxy

echo "Deploying $PROXY_IMAGE to $HOST ($DOMAIN)"

ssh "root@$HOST" "mkdir -p $DIR"
scp -q compose.yaml Caddyfile "root@$HOST:$DIR/"
printf 'DOMAIN=%s\nPROXY_IMAGE=%s\n' "$DOMAIN" "$PROXY_IMAGE" | ssh "root@$HOST" "cat > $DIR/.env"

ssh "root@$HOST" "cd $DIR && docker compose pull -q && docker compose up -d --remove-orphans \
  && docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile \
  && docker image prune -f >/dev/null"

echo "Checking https://$DOMAIN$HEALTH_PATH"
for _ in $(seq 1 30); do
  if curl -fsS -o /dev/null "https://$DOMAIN$HEALTH_PATH"; then
    echo "Healthy"
    exit 0
  fi
  sleep 2
done
echo "Health check failed" >&2
ssh "root@$HOST" "cd $DIR && docker compose ps && docker compose logs --tail 30" >&2
exit 1
