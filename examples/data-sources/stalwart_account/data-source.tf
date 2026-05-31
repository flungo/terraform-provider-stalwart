data "stalwart_account" "alice" {
  email = "alice@example.com"
}

output "alice_quota" {
  value = data.stalwart_account.alice.quota
}
