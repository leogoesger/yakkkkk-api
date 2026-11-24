output "ip" {
  value = digitalocean_droplet.proxy.ipv4_address
}

output "ssh" {
  value = "ssh root@${digitalocean_droplet.proxy.ipv4_address}"
}
