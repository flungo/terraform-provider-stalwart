resource "stalwart_role" "support" {
  description          = "Support team role"
  enabled_permissions  = ["email-send", "email-receive"]
  disabled_permissions = ["settings-list"]
}

# Import an existing role by its description:
# terraform import stalwart_role.support "Support team role"
