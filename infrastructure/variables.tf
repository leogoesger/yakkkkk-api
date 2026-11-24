variable "ssh_key_name" {
  description = "Name of your SSH key in DigitalOcean (Settings > Security)."
  type        = string
}

variable "region" {
  type    = string
  default = "sfo2"
}

variable "size" {
  type    = string
  default = "s-1vcpu-512mb-10gb" # $6/month
}

variable "image" {
  type    = string
  default = "ubuntu-24-04-x64"
}

