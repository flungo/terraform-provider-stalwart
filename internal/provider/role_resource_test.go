// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccRoleResource exercises a stalwart_role through its full lifecycle and
// verifies every writable field in state and on the server. Permission values
// are camelCase JMAP identifiers (e.g. emailSend), matching the server's
// Permission enum serialization.
func TestAccRoleResource(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig("support role",
					[]string{"emailSend", "emailReceive"}, []string{"impersonate"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "support role"),
					resource.TestCheckResourceAttr(resourceName, "enabled_permissions.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "disabled_permissions.#", "1"),
					checkServerRole(c, resourceName, func(r client.Role) error {
						return firstErr(
							wantStr("description", r.Description, "support role"),
							wantSetPtr("enabledPermissions", r.EnabledPermissions, "emailSend", "emailReceive"),
							wantSetPtr("disabledPermissions", r.DisabledPermissions, "impersonate"),
						)
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     "support role",
				ImportStateVerify: true,
			},
			{
				Config: testAccRoleConfig("updated role",
					[]string{"emailSend"}, []string{}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "updated role"),
					resource.TestCheckResourceAttr(resourceName, "enabled_permissions.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "disabled_permissions.#", "0"),
					checkServerRole(c, resourceName, func(r client.Role) error {
						return firstErr(
							wantStr("description", r.Description, "updated role"),
							wantSetPtr("enabledPermissions", r.EnabledPermissions, "emailSend"),
							wantSetPtr("disabledPermissions", r.DisabledPermissions),
						)
					}),
				),
			},
		},
	})
}

func testAccRoleConfig(description string, enabled, disabled []string) string {
	return fmt.Sprintf(`
resource "stalwart_role" "test" {
  description          = %[1]q
  enabled_permissions  = %[2]s
  disabled_permissions = %[3]s
}
`, description, hclStringList(enabled), hclStringList(disabled))
}
