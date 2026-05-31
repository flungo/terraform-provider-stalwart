resource "stalwart_dkim_signature" "example" {
  domain_id   = stalwart_domain.example.id
  selector    = "tf2026"
  algorithm   = "ed25519-sha256" # or "rsa-sha256"
  private_key = var.dkim_private_key
  expiry      = "90d"
}

# Import an existing DKIM signature by its opaque id (ULID):
# terraform import stalwart_dkim_signature.example 01ARZ3NDEKTSV4RRFFQ69G5FAV
