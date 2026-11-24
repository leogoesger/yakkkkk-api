#!/usr/bin/env bash
# Deploys Caddy and the proxy to the server with Docker Compose.
#
#   deployment/deploy.sh --build                 # build app/ on the server and deploy it
#   deployment/deploy.sh                         # redeploy config, keep the current image
#   PROXY_IMAGE=yakkkkk-proxy:v20261124... deployment/deploy.sh   # deploy or roll back to a given image
#   HOST=1.2.3.4 DOMAIN=api.example.com PROXY_IMAGE=ghcr.io/me/repo:sha-abc1234 deployment/deploy.sh   # CI
#
# Uses deployment/.env (DOMAIN) and deployment/proxy.env (app settings) when present.
# List images on the server for rollback: ssh root@HOST docker images yakkkkk-proxy
set -euo pipefail

cd "$(dirname "$0")"
[ -f .env ] && . ./.env

HOST="${HOST:-$(terraform -chdir=../infrastructure output -raw ip)}"
DOMAIN="${DOMAIN:?set DOMAIN in deployment/.env or the environment}"
HEALTH_PATH="${HEALTH_PATH:-/status}"
DIR=/opt/proxy

ssh "root@$HOST" "mkdir -p $DIR"

if [ "${1:-}" = "--build" ]; then
  # Build the Go app on the server; older tags stay there for rollback.
  version="v$(date -u +%Y%m%d%H%M%S)"
  PROXY_IMAGE="yakkkkk-proxy:$version"
  echo "Building $PROXY_IMAGE on $HOST"
  COPYFILE_DISABLE=1 tar --no-xattrs -C ../app -czf - . | ssh "root@$HOST" "rm -rf $DIR/app && mkdir -p $DIR/app && tar -C $DIR/app -xzf -"
  ssh "root@$HOST" "docker build -q --build-arg VERSION=$version -t $PROXY_IMAGE $DIR/app"
fi
# Default: keep the image the server is running now.
PROXY_IMAGE="${PROXY_IMAGE:-$(ssh "root@$HOST" "sed -n 's/^PROXY_IMAGE=//p' $DIR/.env 2>/dev/null")}"
PROXY_IMAGE="${PROXY_IMAGE:?no image: run with --build or set PROXY_IMAGE}"

echo "Deploying $PROXY_IMAGE to $HOST ($DOMAIN)"

# CI passes a short-lived token so the server can pull private GHCR images.
if [ -n "${REGISTRY_TOKEN:-}" ]; then
  echo "$REGISTRY_TOKEN" | ssh "root@$HOST" "docker login ghcr.io -u '${REGISTRY_USER:?}' --password-stdin >/dev/null"
fi
scp -q compose.yaml Caddyfile "root@$HOST:$DIR/"
# App settings and secrets: upload deployment/proxy.env if present, otherwise keep the server's copy.
if [ -f proxy.env ]; then
  ssh "root@$HOST" "umask 077 && cat > $DIR/proxy.env" < proxy.env
else
  ssh "root@$HOST" "touch $DIR/proxy.env && chmod 600 $DIR/proxy.env"
fi
printf 'DOMAIN=%s\nPROXY_IMAGE=%s\n' "$DOMAIN" "$PROXY_IMAGE" | ssh "root@$HOST" "cat > $DIR/.env"

ssh "root@$HOST" "cd $DIR && docker compose pull -q --ignore-pull-failures && docker compose up -d --remove-orphans \
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
