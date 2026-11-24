data "digitalocean_ssh_key" "admin" {
  name = var.ssh_key_name
}

resource "digitalocean_droplet" "proxy" {
  name       = "proxy"
  region     = var.region
  size       = var.size
  image      = var.image
  ssh_keys   = [data.digitalocean_ssh_key.admin.id]
  monitoring = true # free CPU/memory/disk metrics and alerts

  # Runs once on first boot.
  user_data = file("${path.module}/scripts/bootstrap.sh")

  lifecycle {
    prevent_destroy = true
    # Changing these would rebuild the droplet.
    ignore_changes = [image, user_data, ssh_keys]
  }
}
