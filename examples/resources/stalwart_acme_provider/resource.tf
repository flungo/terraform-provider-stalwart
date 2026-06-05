resource "stalwart_acme_provider" "letsencrypt" {
  challenge_type = "TlsAlpn01"
  contact        = ["mailto:admin@example.com"]
  directory      = "https://acme-v02.api.letsencrypt.org/directory"
}

# Import an existing ACME provider by its opaque id:
# terraform import stalwart_acme_provider.letsencrypt <opaque-id>
