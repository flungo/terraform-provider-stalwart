// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccDirectoryResourceLDAP exercises the full lifecycle of a
// stalwart_directory resource with type=Ldap.
func TestAccDirectoryResourceLDAP(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_directory.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with every LDAP field set.
			{
				Config: testAccDirectoryLDAPConfigFull(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "type", "Ldap"),
					resource.TestCheckResourceAttr(resourceName, "description", "Test LDAP directory"),
					resource.TestCheckResourceAttr(resourceName, "url", "ldap://ldap.example.com:389"),
					resource.TestCheckResourceAttr(resourceName, "base_dn", "dc=example,dc=com"),
					resource.TestCheckResourceAttr(resourceName, "bind_dn", "cn=reader,dc=example,dc=com"),
					resource.TestCheckResourceAttr(resourceName, "filter_login", "(&(objectClass=inetOrgPerson)(uid=?))"),
					resource.TestCheckResourceAttr(resourceName, "filter_mailbox", "(&(objectClass=inetOrgPerson)(mail=?))"),
					resource.TestCheckResourceAttr(resourceName, "attr_email.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "attr_email.*", "mail"),
					resource.TestCheckResourceAttr(resourceName, "attr_member_of.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "attr_member_of.*", "memberOf"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					checkServerDirectory(c, resourceName, func(d client.Directory) error {
						return firstErr(
							wantStr("@type", d.Type, "Ldap"),
							wantStr("url", d.URL, "ldap://ldap.example.com:389"),
							wantStr("baseDn", d.BaseDN, "dc=example,dc=com"),
							wantStr("bindDn", d.BindDN, "cn=reader,dc=example,dc=com"),
							wantSet("attrEmail", d.AttrEmail, "mail"),
							wantSet("attrMemberOf", d.AttrMemberOf, "memberOf"),
						)
					}),
				),
			},
			// Import by id.
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bind_secret"},
			},
			// Update mutable fields.
			{
				Config: testAccDirectoryLDAPConfigUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "Updated LDAP directory"),
					resource.TestCheckResourceAttr(resourceName, "attr_email.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "attr_email.*", "mail"),
					resource.TestCheckTypeSetElemAttr(resourceName, "attr_email.*", "proxyAddresses"),
					checkServerDirectory(c, resourceName, func(d client.Directory) error {
						return wantSet("attrEmail", d.AttrEmail, "mail", "proxyAddresses")
					}),
				),
			},
		},
	})
}

// TestAccDirectoryResourceOIDC exercises the full lifecycle of a
// stalwart_directory resource with type=Oidc.
func TestAccDirectoryResourceOIDC(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_directory.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryOIDCConfigFull(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "type", "Oidc"),
					resource.TestCheckResourceAttr(resourceName, "description", "Test OIDC directory"),
					resource.TestCheckResourceAttr(resourceName, "issuer_url", "https://accounts.google.com"),
					resource.TestCheckResourceAttr(resourceName, "claim_username", "email"),
					resource.TestCheckResourceAttr(resourceName, "require_scopes.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "require_scopes.*", "email"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					checkServerDirectory(c, resourceName, func(d client.Directory) error {
						return firstErr(
							wantStr("@type", d.Type, "Oidc"),
							wantStr("issuerUrl", d.IssuerURL, "https://accounts.google.com"),
							wantStr("claimUsername", d.ClaimUsername, "email"),
							wantSet("requireScopes", d.RequireScopes, "email"),
						)
					}),
				),
			},
			// Import by id.
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update mutable OIDC fields.
			{
				Config: testAccDirectoryOIDCConfigUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "Updated OIDC directory"),
					resource.TestCheckResourceAttr(resourceName, "require_scopes.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "require_scopes.*", "email"),
					resource.TestCheckTypeSetElemAttr(resourceName, "require_scopes.*", "openid"),
					checkServerDirectory(c, resourceName, func(d client.Directory) error {
						return wantSet("requireScopes", d.RequireScopes, "email", "openid")
					}),
				),
			},
		},
	})
}

func testAccDirectoryLDAPConfigFull() string {
	return `
resource "stalwart_directory" "test" {
  type           = "Ldap"
  description    = "Test LDAP directory"
  url            = "ldap://ldap.example.com:389"
  base_dn        = "dc=example,dc=com"
  bind_dn        = "cn=reader,dc=example,dc=com"
  bind_secret    = "secret-bind-password"
  filter_login   = "(&(objectClass=inetOrgPerson)(uid=?))"
  filter_mailbox = "(&(objectClass=inetOrgPerson)(mail=?))"
  attr_email     = ["mail"]
  attr_member_of = ["memberOf"]
}
`
}

func testAccDirectoryLDAPConfigUpdated() string {
	return `
resource "stalwart_directory" "test" {
  type           = "Ldap"
  description    = "Updated LDAP directory"
  url            = "ldap://ldap.example.com:389"
  base_dn        = "dc=example,dc=com"
  bind_dn        = "cn=reader,dc=example,dc=com"
  bind_secret    = "secret-bind-password"
  filter_login   = "(&(objectClass=inetOrgPerson)(uid=?))"
  filter_mailbox = "(&(objectClass=inetOrgPerson)(mail=?))"
  attr_email     = ["mail", "proxyAddresses"]
  attr_member_of = ["memberOf"]
}
`
}

func testAccDirectoryOIDCConfigFull() string {
	return `
resource "stalwart_directory" "test" {
  type           = "Oidc"
  description    = "Test OIDC directory"
  issuer_url     = "https://accounts.google.com"
  claim_username = "email"
  require_scopes = ["email"]
}
`
}

func testAccDirectoryOIDCConfigUpdated() string {
	return `
resource "stalwart_directory" "test" {
  type           = "Oidc"
  description    = "Updated OIDC directory"
  issuer_url     = "https://accounts.google.com"
  claim_username = "email"
  require_scopes = ["email", "openid"]
}
`
}
