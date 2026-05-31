data "stalwart_dns_records" "example" {
  domain = "example.com"
}

# The zone_file contains the DNS records Stalwart expects to be published for
# the domain (DKIM, SPF, MX, DMARC, MTA-STS, etc.).
output "dns_zone_file" {
  value = data.stalwart_dns_records.example.zone_file
}
