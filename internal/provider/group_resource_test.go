// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccGroupResource exercises a stalwart_group through its full lifecycle and
// verifies every writable field in state and on the server.
func TestAccGroupResource(t *testing.T) {
	c := accClient(t)
	const domain = "tf-acc-group.example"
	const resourceName = "stalwart_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig(domain, "All staff", "Custom"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "team"),
					resource.TestCheckResourceAttr(resourceName, "description", "All staff"),
					resource.TestCheckResourceAttr(resourceName, "role", "Custom"),
					resource.TestCheckResourceAttr(resourceName, "email_address", "team@"+domain),
					resource.TestCheckResourceAttr(resourceName, "role_ids.#", "1"),
					checkServerAccount(c, resourceName, func(a client.Account) error {
						return firstErr(
							wantStr("@type", a.Type, client.AccountTypeGroup),
							wantStr("name", a.Name, "team"),
							wantStr("description", a.Description, "All staff"),
							wantStr("emailAddress", a.EmailAddress, "team@"+domain),
							wantRoleType(a.Roles, "Custom"),
						)
					}),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           "team@" + domain,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"domain"},
			},
			{
				Config: testAccGroupConfigDefault(domain, "Updated staff"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "Updated staff"),
					resource.TestCheckResourceAttr(resourceName, "role", "Default"),
					checkServerAccount(c, resourceName, func(a client.Account) error {
						return firstErr(
							wantStr("description", a.Description, "Updated staff"),
							wantRoleType(a.Roles, "Default"),
						)
					}),
				),
			},
		},
	})
}

func testAccGroupConfig(domain, description, _ string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_role" "support" {
  description = "support role for %[1]s"
}

resource "stalwart_group" "test" {
  domain_id   = stalwart_domain.test.id
  name        = "team"
  description = %[2]q
  role        = "Custom"
  role_ids    = [stalwart_role.support.id]
}
`, domain, description)
}

func testAccGroupConfigDefault(domain, description string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_role" "support" {
  description = "support role for %[1]s"
}

resource "stalwart_group" "test" {
  domain_id   = stalwart_domain.test.id
  name        = "team"
  description = %[2]q
  role        = "Default"
}
`, domain, description)
}
