data "stalwart_domain" "example" {
  name = "example.com"
}

output "domain_id" {
  value = data.stalwart_domain.example.id
}
