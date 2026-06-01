resource "stalwart_role" "support" {
  description          = "Support team role"
  enabled_permissions  = ["emailSend", "emailReceive"]
  disabled_permissions = ["impersonate"]
}

# Import an existing role by its description:
# terraform import stalwart_role.support "Support team role"
