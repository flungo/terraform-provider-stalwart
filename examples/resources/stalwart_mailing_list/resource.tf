resource "stalwart_mailing_list" "announce" {
  domain_id   = stalwart_domain.example.id
  name        = "announce"
  description = "Company announcements"
  recipients = [
    "alice@example.com",
    "bob@example.com",
  ]
}

# Import an existing mailing list by its email address:
# terraform import stalwart_mailing_list.announce announce@example.com
