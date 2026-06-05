resource "stalwart_network_listener" "smtp" {
  name     = "smtp"
  bind     = ["0.0.0.0:25"]
  protocol = "smtp"
}

resource "stalwart_network_listener" "imaps" {
  name         = "imaps"
  bind         = ["0.0.0.0:993"]
  protocol     = "imap"
  tls_implicit = true
}

# Import an existing listener by its name or opaque id:
# terraform import stalwart_network_listener.smtp smtp
