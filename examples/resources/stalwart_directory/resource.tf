resource "stalwart_directory" "ldap" {
  type        = "Ldap"
  description = "Corporate LDAP directory"

  url         = "ldap://auth.corp.example.com:389"
  base_dn     = "dc=corp,dc=example,dc=com"
  bind_dn     = "cn=readonly,dc=corp,dc=example,dc=com"
  bind_secret = var.ldap_bind_secret

  filter_login   = "(&(objectClass=inetOrgPerson)(uid=?))"
  filter_mailbox = "(&(objectClass=inetOrgPerson)(mail=?))"
  attr_email     = ["mail"]
}

# OIDC example:
# resource "stalwart_directory" "oidc" {
#   type           = "Oidc"
#   description    = "Corporate SSO"
#   issuer_url     = "https://sso.corp.example.com"
#   claim_username = "preferred_username"
# }

# Import an existing directory by its opaque id:
# terraform import stalwart_directory.ldap <opaque-id>
