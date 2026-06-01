// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccMailingListResource exercises a stalwart_mailing_list through its full
// lifecycle and verifies every writable field in state and on the server.
func TestAccMailingListResource(t *testing.T) {
	c := accClient(t)
	const domain = "tf-acc-list.example"
	const resourceName = "stalwart_mailing_list.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMailingListConfig(domain, "Announcements",
					[]string{"alice@" + domain, "bob@" + domain}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "announce"),
					resource.TestCheckResourceAttr(resourceName, "description", "Announcements"),
					resource.TestCheckResourceAttr(resourceName, "email_address", "announce@"+domain),
					resource.TestCheckResourceAttr(resourceName, "recipients.#", "2"),
					checkServerMailingList(c, resourceName, func(l client.MailingList) error {
						return firstErr(
							wantStr("name", l.Name, "announce"),
							wantStr("description", l.Description, "Announcements"),
							wantStr("emailAddress", l.EmailAddress, "announce@"+domain),
							wantSet("recipients", l.Recipients, "alice@"+domain, "bob@"+domain),
						)
					}),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           "announce@" + domain,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"domain"},
			},
			{
				Config: testAccMailingListConfig(domain, "Updated list",
					[]string{"carol@" + domain}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "Updated list"),
					resource.TestCheckResourceAttr(resourceName, "recipients.#", "1"),
					checkServerMailingList(c, resourceName, func(l client.MailingList) error {
						return firstErr(
							wantStr("description", l.Description, "Updated list"),
							wantSet("recipients", l.Recipients, "carol@"+domain),
						)
					}),
				),
			},
		},
	})
}

func testAccMailingListConfig(domain, description string, recipients []string) string {
	quoted := ""
	for i, r := range recipients {
		if i > 0 {
			quoted += ", "
		}
		quoted += fmt.Sprintf("%q", r)
	}
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_mailing_list" "test" {
  domain_id   = stalwart_domain.test.id
  name        = "announce"
  description = %[2]q
  recipients  = [%[3]s]
}
`, domain, description, quoted)
}
