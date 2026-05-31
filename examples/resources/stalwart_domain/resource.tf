resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Primary mail domain"
  catchall    = "postmaster@example.com"
  aliases     = ["example.net"]
}

# Import an existing domain by its name:
# terraform import stalwart_domain.example example.com
