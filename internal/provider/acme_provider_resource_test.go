// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccAcmeProviderResource exercises the full lifecycle of a
// stalwart_acme_provider resource and verifies fields both via Terraform state
// and direct JMAP client.
//
// max_retries exists only on v0.16 and a settable description only on v1, so
// the "full" configuration sets whichever the server under test has.
func TestAccAcmeProviderResource(t *testing.T) {
	c := accClient(t)
	v1 := accServerVersion(t) == client.Version1
	const resourceName = "stalwart_acme_provider.test"

	// Version-specific checks for the create step and the update step (where
	// the attribute is omitted and must keep its prior value).
	var versioned resource.TestCheckFunc
	if v1 {
		versioned = resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "description", "Let's Encrypt staging"),
			resource.TestCheckNoResourceAttr(resourceName, "max_retries"),
			checkServerAcmeProvider(c, resourceName, func(p client.AcmeProvider) error {
				return wantStr("description", p.Description, "Let's Encrypt staging")
			}),
		)
	} else {
		versioned = resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "max_retries", "3"),
			// Server-derived on v0.16: the directory URL, plus the account id.
			resource.TestMatchResourceAttr(resourceName, "description",
				regexp.MustCompile(`^https://acme-staging-v02\.api\.letsencrypt\.org/directory`)),
		)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with every field set.
			{
				Config: testAccAcmeProviderConfigFull(v1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "challenge_type", "Dns01"),
					resource.TestCheckResourceAttr(resourceName, "directory", "https://acme-staging-v02.api.letsencrypt.org/directory"),
					resource.TestCheckResourceAttr(resourceName, "contact.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "contact.*", "mailto:acme@stalwart-tf-acc.net"),
					resource.TestCheckResourceAttr(resourceName, "renew_before", "R23"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					versioned,
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
					// renew_before, max_retries and description are Optional+Computed;
					// omitting them from config preserves the prior value.
					resource.TestCheckResourceAttr(resourceName, "renew_before", "R23"),
					versioned,
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

func testAccAcmeProviderConfigFull(v1 bool) string {
	versioned := `max_retries    = 3`
	if v1 {
		versioned = `description    = "Let's Encrypt staging"`
	}
	return `
resource "stalwart_acme_provider" "test" {
  challenge_type = "Dns01"
  directory      = "https://acme-staging-v02.api.letsencrypt.org/directory"
  contact        = ["mailto:acme@stalwart-tf-acc.net"]
  renew_before   = "R23"
  ` + versioned + `
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
