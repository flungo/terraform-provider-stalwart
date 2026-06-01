terraform {
  required_providers {
    stalwart = {
      source = "flungo/stalwart"
    }
  }
}

provider "stalwart" {
  endpoint = "https://mail.example.com" # base URL; the provider appends /jmap
  token    = var.stalwart_token         # or set STALWART_TOKEN

  # Username/password authentication is an alternative to a bearer token:
  # username = "admin"
  # password = var.stalwart_password
}

variable "stalwart_token" {
  type      = string
  sensitive = true
}
