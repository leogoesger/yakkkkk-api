# Proxy

## Infrastructure (DigitalOcean)

Creates a $6/month droplet in `sfo3` with a Cloud Firewall (22, 80, 443 open; SSH is key-only).
`scripts/bootstrap.sh` runs automatically on first boot and installs Docker.

```bash
export DIGITALOCEAN_TOKEN=your-token   # never commit this
cd infrastructure
cp terraform.tfvars.example terraform.tfvars   # fill in
terraform init
terraform plan -out=tfplan    # check: 2 to add, 0 to destroy
terraform apply tfplan
```

To re-run the bootstrap on an existing server:

```bash
ssh root@SERVER_IP 'bash -s' < infrastructure/scripts/bootstrap.sh
```

## Deployment

`deployment/deploy.sh` copies `compose.yaml`, `Caddyfile`, and `proxy.env` (if present) to `/opt/proxy` on the server,
starts the containers, reloads Caddy, and checks `https://$DOMAIN`.

```bash
cp deployment/.env.example deployment/.env   # DOMAIN=api.yakkkkk.com
cp deployment/proxy.env.example deployment/proxy.env   # app secrets
deployment/deploy.sh --build                  # build app/ on the server and deploy it
deployment/deploy.sh                          # redeploy config, keep current image
PROXY_IMAGE=ghcr.io/OWNER/proxy:TAG deployment/deploy.sh   # deploy or roll back a version
```

## Proxy app

Requests to `https://api.yakkkkk.com/<route>/<path>` are forwarded to that route's upstream with the
upstream API key added. There is no client authentication: anyone with the URL can use the
configured upstreams. Configuration: `deployment/proxy.env.example`.

The Go code lives in `app/`:

```bash
cd app
go test -race ./...
ROUTES=httpbin=https://httpbin.org go run ./cmd/api
```

## CI/CD (GitHub Actions + GHCR)

- `.github/workflows/ci.yml`: gofmt, `go vet`, and race tests on pull requests.
- `.github/workflows/deploy.yml`: on push to `main`, tests, builds `ghcr.io/<owner>/<repo>:sha-<commit>`,
  pushes it to GHCR, and runs `deployment/deploy.sh` against the server. Infrastructure is never touched.
- Rollback: Actions → Deploy → Run workflow, with `tag` set to an earlier `sha-...` tag.

Repository secrets: `DEPLOY_HOST` (server IP) and `DEPLOY_SSH_KEY` (a private key whose public key is in
the server's `/root/.ssh/authorized_keys`). Optional repository variable: `DOMAIN` (default `api.yakkkkk.com`).
App settings in `/opt/proxy/proxy.env` on the server are kept between deploys.
