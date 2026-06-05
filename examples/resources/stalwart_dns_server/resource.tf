resource "stalwart_dns_server" "cloudflare" {
  type        = "Cloudflare"
  description = "Cloudflare DNS for automatic record management"
  secret      = var.cloudflare_api_token
}

# RFC 2136 TSIG example:
# resource "stalwart_dns_server" "tsig" {
#   type        = "Tsig"
#   description = "Internal BIND server via TSIG"
#   host        = "10.0.0.53"
#   key_name    = "tsig-key."
#   key         = var.tsig_key
# }

# Import an existing DNS server by its opaque id:
# terraform import stalwart_dns_server.cloudflare <opaque-id>
