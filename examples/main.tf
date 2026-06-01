# A basic, working example: a domain with a DKIM signing key, an individual
# account, a group the account belongs to, a mailing list, and a custom role.
#
# Run with:
#   export STALWART_ENDPOINT=https://mail.example.com
#   export STALWART_TOKEN=...
#   terraform init && terraform apply

terraform {
  required_providers {
    stalwart = {
      source = "flungo/stalwart"
    }
  }
}

# Endpoint and credentials are taken from STALWART_ENDPOINT / STALWART_TOKEN.
provider "stalwart" {}

resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Primary mail domain, managed by Terraform"
  catchall    = "postmaster@example.com"
}

resource "stalwart_dkim_signature" "example" {
  domain_id   = stalwart_domain.example.id
  selector    = "tf2026"
  algorithm   = "ed25519-sha256"
  private_key = var.dkim_private_key
}

resource "stalwart_role" "support" {
  description         = "Support team role"
  enabled_permissions = ["emailSend", "emailReceive"]
}

resource "stalwart_group" "team" {
  domain_id   = stalwart_domain.example.id
  name        = "team"
  description = "All staff"
}

resource "stalwart_account" "alice" {
  domain_id   = stalwart_domain.example.id
  name        = "alice"
  description = "Alice Example"
  password    = var.alice_password
  quota       = 5 * 1024 * 1024 * 1024 # 5 GiB
  role        = "Custom"
  role_ids    = [stalwart_role.support.id]
  member_of   = [stalwart_group.team.id]
}

resource "stalwart_mailing_list" "announce" {
  domain_id  = stalwart_domain.example.id
  name       = "announce"
  recipients = [stalwart_account.alice.email_address]
}

# Read the DNS records the server expects to be published for the domain.
data "stalwart_dns_records" "example" {
  domain = stalwart_domain.example.name
}

output "dns_zone_file" {
  value = data.stalwart_dns_records.example.zone_file
}

variable "dkim_private_key" {
  type      = string
  sensitive = true
}

variable "alice_password" {
  type      = string
  sensitive = true
}
