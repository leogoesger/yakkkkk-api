#!/usr/bin/env bash
# Prepares an Ubuntu or Debian server for the proxy. Safe to run more than once.
# Existing server: ssh root@SERVER 'bash -s' < infrastructure/scripts/bootstrap.sh
# New servers: Terraform passes this file as user_data; cloud-init runs it on first boot.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

# System packages
rm -f /etc/apt/sources.list.d/docker.sources /etc/apt/keyrings/docker.asc
apt-get update
apt-get -y upgrade
apt-get -y install ca-certificates curl unattended-upgrades

# 1 GB swap so small droplets (512 MB) can build images without running out of memory
if ! swapon --show | grep -q /swapfile; then
  [ -f /swapfile ] || { fallocate -l 1G /swapfile && chmod 600 /swapfile && mkswap /swapfile; }
  swapon /swapfile
fi
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab

# Automatic security updates
cat > /etc/apt/apt.conf.d/20auto-upgrades <<'CONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
CONF

# SSH: keys only (root keeps key login)
cat > /etc/ssh/sshd_config.d/10-hardening.conf <<'CONF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
CONF
sshd -t
systemctl try-reload-or-restart ssh

# Docker Engine and Compose, from the distribution's packages
. /etc/os-release
if [ "$ID" = ubuntu ]; then
  apt-get -y install docker.io docker-compose-v2 docker-buildx
else
  apt-get -y install docker.io docker-compose
fi

# Docker log rotation
daemon_json='{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}'
if [ "$(cat /etc/docker/daemon.json 2>/dev/null)" != "$daemon_json" ]; then
  echo "$daemon_json" > /etc/docker/daemon.json
  systemctl restart docker
fi

# Start Docker on boot
systemctl enable --now docker

echo "Bootstrap complete: $(docker --version), $(docker compose version)"
