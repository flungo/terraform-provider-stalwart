resource "stalwart_group" "team" {
  domain_id   = stalwart_domain.example.id
  name        = "team"
  description = "All staff"
}

# Membership is managed from the account side via member_of:
resource "stalwart_account" "alice" {
  domain_id = stalwart_domain.example.id
  name      = "alice"
  member_of = [stalwart_group.team.id]
}

# Import an existing group by its email address:
# terraform import stalwart_group.team team@example.com
