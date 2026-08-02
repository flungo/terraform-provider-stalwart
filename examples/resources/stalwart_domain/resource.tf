resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Primary mail domain"
  catchall    = "postmaster@example.com"
  aliases     = ["example.net"]
}

# Automatic certificate management, using an ACME provider and DNS server
# defined elsewhere. Automatic DNS management is required alongside an ACME
# provider: the DNS-01 challenge is answered by publishing records into the
# domain's own zone, so Stalwart must be able to write them.
#
# The order of subject_alternative_names is significant: the first entry becomes
# the issued certificate's Subject Common Name, so list the hostname clients
# connect to first. Bare hostnames have the domain appended; entries containing
# a dot are used as-is.
resource "stalwart_domain" "managed_certificate" {
  name     = "example.org"
  catchall = "postmaster@example.org"

  certificate_management    = "Automatic"
  acme_provider_id          = stalwart_acme_provider.example.id
  subject_alternative_names = ["mail", "autoconfig", "autodiscover"]

  dns_management = "Automatic"
  dns_server_id  = stalwart_dns_server.example.id
}

# Import an existing domain by its name:
# terraform import stalwart_domain.example example.com
