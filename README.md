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

`deployment/deploy.sh` copies `compose.yaml` and `Caddyfile` to `/opt/proxy` on the server,
starts the containers, reloads Caddy, and checks `https://$DOMAIN`.

```bash
cp deployment/.env.example deployment/.env   # DOMAIN=api.yakkkkk.com
deployment/deploy.sh                          # host comes from terraform output
PROXY_IMAGE=ghcr.io/OWNER/proxy:TAG deployment/deploy.sh   # deploy or roll back a version
```
