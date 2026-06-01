// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDomainDataSource verifies that the domain data source reads a domain by
// name and exposes its fields.
func TestAccDomainDataSource(t *testing.T) {
	const domain = "tf-acc-ds-domain.test"
	const dsName = "data.stalwart_domain.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name        = %[1]q
  description = "data source domain"
  catchall    = "postmaster@%[1]s"
}

data "stalwart_domain" "test" {
  name = stalwart_domain.test.name
}
`, domain),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsName, "name", domain),
					resource.TestCheckResourceAttr(dsName, "description", "data source domain"),
					resource.TestCheckResourceAttr(dsName, "catchall", "postmaster@"+domain),
					resource.TestCheckResourceAttrSet(dsName, "id"),
					resource.TestCheckResourceAttrSet(dsName, "dns_zone_file"),
					// The data source id must match the managed resource id.
					resource.TestCheckResourceAttrPair(dsName, "id", "stalwart_domain.test", "id"),
				),
			},
		},
	})
}

// TestAccAccountDataSource verifies that the account data source reads an account
// by email address.
func TestAccAccountDataSource(t *testing.T) {
	const domain = "tf-acc-ds-account.test"
	const dsName = "data.stalwart_account.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_account" "test" {
  domain_id   = stalwart_domain.test.id
  name        = "alice"
  description = "data source account"
  password    = "correct-horse-battery-staple-42"
  quota       = 1073741824
}

data "stalwart_account" "test" {
  email = stalwart_account.test.email_address
}
`, domain),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsName, "name", "alice"),
					resource.TestCheckResourceAttr(dsName, "type", "User"),
					resource.TestCheckResourceAttr(dsName, "description", "data source account"),
					resource.TestCheckResourceAttr(dsName, "email_address", "alice@"+domain),
					resource.TestCheckResourceAttr(dsName, "quota", "1073741824"),
					resource.TestCheckResourceAttrPair(dsName, "id", "stalwart_account.test", "id"),
				),
			},
		},
	})
}

// TestAccDNSRecordsDataSource verifies that the DNS records data source returns
// the domain's zone file.
func TestAccDNSRecordsDataSource(t *testing.T) {
	const domain = "tf-acc-ds-dns.test"
	const dsName = "data.stalwart_dns_records.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

data "stalwart_dns_records" "test" {
  domain = stalwart_domain.test.name
}
`, domain),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsName, "zone_file"),
					resource.TestCheckResourceAttrPair(dsName, "domain_id", "stalwart_domain.test", "id"),
				),
			},
		},
	})
}
