resource "stalwart_account" "alice" {
  domain_id   = stalwart_domain.example.id
  name        = "alice"
  description = "Alice Example"
  password    = var.alice_password
  quota       = 5368709120 # 5 GiB in bytes
}

# Import an existing account by its email address:
# terraform import stalwart_account.alice alice@example.com
