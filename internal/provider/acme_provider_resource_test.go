// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccAcmeProviderResource exercises the full lifecycle of a
// stalwart_acme_provider resource and verifies fields both via Terraform state
// and direct JMAP client.
func TestAccAcmeProviderResource(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_acme_provider.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with every field set.
			{
				Config: testAccAcmeProviderConfigFull(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "challenge_type", "Dns01"),
					resource.TestCheckResourceAttr(resourceName, "directory", "https://acme-staging-v02.api.letsencrypt.org/directory"),
					resource.TestCheckResourceAttr(resourceName, "contact.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "contact.*", "mailto:acme@stalwart-tf-acc.net"),
					resource.TestCheckResourceAttr(resourceName, "renew_before", "R23"),
					resource.TestCheckResourceAttr(resourceName, "max_retries", "3"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					checkServerAcmeProvider(c, resourceName, func(p client.AcmeProvider) error {
						return firstErr(
							wantStr("challengeType", p.ChallengeType, "Dns01"),
							wantStr("directory", p.Directory, "https://acme-staging-v02.api.letsencrypt.org/directory"),
							wantSet("contact", p.Contact, "mailto:acme@stalwart-tf-acc.net"),
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
			// Update mutable fields.
			{
				Config: testAccAcmeProviderConfigUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "contact.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "contact.*", "mailto:acme@stalwart-tf-acc.net"),
					resource.TestCheckTypeSetElemAttr(resourceName, "contact.*", "mailto:admin@stalwart-tf-acc.net"),
					// renew_before and max_retries are Optional+Computed; omitting from config
					// preserves the prior state value via UseStateForUnknown.
					resource.TestCheckResourceAttr(resourceName, "renew_before", "R23"),
					resource.TestCheckResourceAttr(resourceName, "max_retries", "3"),
					checkServerAcmeProvider(c, resourceName, func(p client.AcmeProvider) error {
						return wantSet("contact", p.Contact,
							"mailto:acme@stalwart-tf-acc.net",
							"mailto:admin@stalwart-tf-acc.net",
						)
					}),
				),
			},
		},
	})
}

func testAccAcmeProviderConfigFull() string {
	return `
resource "stalwart_acme_provider" "test" {
  challenge_type = "Dns01"
  directory      = "https://acme-staging-v02.api.letsencrypt.org/directory"
  contact        = ["mailto:acme@stalwart-tf-acc.net"]
  renew_before   = "R23"
  max_retries    = 3
}
`
}

func testAccAcmeProviderConfigUpdated() string {
	return `
resource "stalwart_acme_provider" "test" {
  challenge_type = "Dns01"
  directory      = "https://acme-staging-v02.api.letsencrypt.org/directory"
  contact        = ["mailto:acme@stalwart-tf-acc.net", "mailto:admin@stalwart-tf-acc.net"]
}
`
}
