// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccAccountResource exercises a stalwart_account through its full lifecycle
// and verifies every writable field in Terraform state and on the server.
func TestAccAccountResource(t *testing.T) {
	c := accClient(t)
	const domain = "tf-acc-account.test"
	const resourceName = "stalwart_account.test"
	const groupName = "stalwart_group.team"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountConfig(domain, "Alice Example", 5368709120, "alice"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "alice"),
					resource.TestCheckResourceAttr(resourceName, "description", "Alice Example"),
					resource.TestCheckResourceAttr(resourceName, "quota", "5368709120"),
					resource.TestCheckResourceAttr(resourceName, "role", "Custom"),
					resource.TestCheckResourceAttr(resourceName, "email_address", "alice@"+domain),
					resource.TestCheckResourceAttr(resourceName, "member_of.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "role_ids.#", "1"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					// Independent server verification, including the id linkages
					// (domainId, memberGroupIds, role ids) resolved from state.
					checkServerAccount(c, resourceName, func(a client.Account) error {
						return firstErr(
							wantStr("@type", a.Type, client.AccountTypeUser),
							wantStr("name", a.Name, "alice"),
							wantStr("description", a.Description, "Alice Example"),
							wantStr("emailAddress", a.EmailAddress, "alice@"+domain),
							wantQuota("maxDiskQuota", a.Quotas, 5368709120),
							wantRoleType(a.Roles, "Custom"),
						)
					}),
					checkAccountLinksResolved(c, resourceName, groupName, "stalwart_role.support"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           "alice@" + domain,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "domain"},
			},
			// Update mutable fields: description, quota, role back to User.
			{
				Config: testAccAccountConfigSimpleRole(domain, "Alice Updated", 1073741824),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "Alice Updated"),
					resource.TestCheckResourceAttr(resourceName, "quota", "1073741824"),
					resource.TestCheckResourceAttr(resourceName, "role", "User"),
					checkServerAccount(c, resourceName, func(a client.Account) error {
						return firstErr(
							wantStr("description", a.Description, "Alice Updated"),
							wantQuota("maxDiskQuota", a.Quotas, 1073741824),
							wantRoleType(a.Roles, "User"),
						)
					}),
				),
			},
		},
	})
}

// testAccAccountConfig defines a domain, a role, a group, and a custom-role
// account that belongs to the group — exercising role_ids and member_of.
func testAccAccountConfig(domain, description string, quota int64, name string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_role" "support" {
  description = "support role for %[1]s"
}

resource "stalwart_group" "team" {
  domain_id = stalwart_domain.test.id
  name      = "team"
}

resource "stalwart_account" "test" {
  domain_id   = stalwart_domain.test.id
  name        = %[4]q
  description = %[2]q
  password    = "correct-horse-battery-staple-42"
  quota       = %[3]d
  role        = "Custom"
  role_ids    = [stalwart_role.support.id]
  member_of   = [stalwart_group.team.id]
}
`, domain, description, quota, name)
}

// testAccAccountConfigSimpleRole drops the custom role/membership and uses the
// built-in User role, to exercise updates that clear collections.
func testAccAccountConfigSimpleRole(domain, description string, quota int64) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_role" "support" {
  description = "support role for %[1]s"
}

resource "stalwart_group" "team" {
  domain_id = stalwart_domain.test.id
  name      = "team"
}

resource "stalwart_account" "test" {
  domain_id   = stalwart_domain.test.id
  name        = "alice"
  description = %[2]q
  password    = "correct-horse-battery-staple-42"
  quota       = %[3]d
  role        = "User"
}
`, domain, description, quota)
}
