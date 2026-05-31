// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDomainResource exercises the full lifecycle of a stalwart_domain
// resource: create, read, update, import, and (implicit) delete. It requires a
// live Stalwart instance and only runs when TF_ACC is set.
//
// NOTE: This is a representative acceptance test that establishes the pattern
// for the remaining resources. Extend it (and add equivalents for the other
// resources and data sources) once pointed at your test instance.
func TestAccDomainResource(t *testing.T) {
	const domainName = "tf-acc-test.example.com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and read.
			{
				Config: testAccDomainResourceConfig(domainName, "managed by terraform"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stalwart_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("stalwart_domain.test", "description", "managed by terraform"),
					resource.TestCheckResourceAttrSet("stalwart_domain.test", "id"),
				),
			},
			// Import.
			{
				ResourceName:      "stalwart_domain.test",
				ImportState:       true,
				ImportStateId:     domainName,
				ImportStateVerify: true,
				// dns_zone_file and created_at are server-generated and may
				// drift between the import and refresh, so ignore them.
				ImportStateVerifyIgnore: []string{"dns_zone_file"},
			},
			// Update and read.
			{
				Config: testAccDomainResourceConfig(domainName, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stalwart_domain.test", "description", "updated description"),
				),
			},
		},
	})
}

func testAccDomainResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
}
