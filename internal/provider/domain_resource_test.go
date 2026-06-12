// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccDomainResource exercises the full lifecycle of a stalwart_domain
// resource (create, read, update, import, delete) and verifies every writable
// field — both as Terraform records it in state and as the server actually
// persisted it (read back through a direct JMAP client, independent of the
// provider's own read path).
func TestAccDomainResource(t *testing.T) {
	c := accClient(t)
	const name = "tf-acc-domain.test"
	const resourceName = "stalwart_domain.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with every writable field set to a non-default value.
			{
				Config: testAccDomainConfigFull(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state.
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "catchall", "postmaster@"+name),
					resource.TestCheckResourceAttr(resourceName, "aliases.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "aliases.*", "alias1."+name),
					resource.TestCheckTypeSetElemAttr(resourceName, "aliases.*", "alias2."+name),
					resource.TestCheckResourceAttr(resourceName, "enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "subaddressing", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "allow_relaying", "true"),
					resource.TestCheckResourceAttr(resourceName, "report_address", "mailto:dmarc@"+name),
					resource.TestCheckResourceAttr(resourceName, "dkim_management", "Manual"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					// Independent server-side verification of every field.
					checkServerDomain(c, resourceName, func(d client.Domain) error {
						return firstErr(
							wantStr("name", d.Name, name),
							wantStr("description", d.Description, "initial description"),
							wantStr("catchAllAddress", d.CatchAllAddress, "postmaster@"+name),
							wantSet("aliases", d.Aliases, "alias1."+name, "alias2."+name),
							wantBool("isEnabled", d.IsEnabled, false),
							wantBool("allowRelaying", d.AllowRelaying, true),
							wantStr("reportAddressUri", d.ReportAddressURI, "mailto:dmarc@"+name),
							wantVariant("subAddressing", d.SubAddressing, "Disabled"),
							wantVariant("dkimManagement", d.DkimManagement, "Manual"),
						)
					}),
				),
			},
			// Import and verify state round-trips.
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           name,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dns_zone_file"},
			},
			// Update every mutable field to a different value.
			{
				Config: testAccDomainConfigUpdated(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "catchall", "hostmaster@"+name),
					resource.TestCheckResourceAttr(resourceName, "aliases.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "aliases.*", "alias3."+name),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "allow_relaying", "false"),
					checkServerDomain(c, resourceName, func(d client.Domain) error {
						return firstErr(
							wantStr("description", d.Description, "updated description"),
							wantStr("catchAllAddress", d.CatchAllAddress, "hostmaster@"+name),
							wantSet("aliases", d.Aliases, "alias3."+name),
							wantBool("isEnabled", d.IsEnabled, true),
							wantBool("allowRelaying", d.AllowRelaying, false),
						)
					}),
				),
			},
		},
	})
}

// TestAccDomainResourceMinimal verifies a domain created with only its required
// field: server defaults must be reflected without triggering an inconsistent
// plan, and optional-unset fields must read back as null/empty.
func TestAccDomainResourceMinimal(t *testing.T) {
	c := accClient(t)
	const name = "tf-acc-domain-min.test"
	const resourceName = "stalwart_domain.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "stalwart_domain" "test" { name = %q }`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Server-applied defaults surface as Computed values.
					resource.TestCheckResourceAttr(resourceName, "report_address", "mailto:postmaster"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "aliases.#", "0"),
					checkServerDomain(c, resourceName, func(d client.Domain) error {
						return firstErr(
							wantStr("reportAddressUri", d.ReportAddressURI, "mailto:postmaster"),
							wantBool("isEnabled", d.IsEnabled, true),
							wantStrNil("description", d.Description),
							wantStrNil("catchAllAddress", d.CatchAllAddress),
							wantSet("aliases", d.Aliases),
						)
					}),
				),
			},
		},
	})
}

// TestAccDomainResourceAutomaticDNS exercises a domain with Automatic DNS
// management and a populated publish_records set. This is the regression
// scenario that previously broke read/import/plan/apply: Stalwart returns
// publishRecords as a Map<DnsRecordType> object ({"dkim":true,...}), which the
// provider once modeled as a bool. Create + read + import are all covered so
// the round-trip through the publishRecords set is verified end to end, both in
// Terraform state and via an independent server read.
func TestAccDomainResourceAutomaticDNS(t *testing.T) {
	c := accClient(t)
	const name = "tf-acc-domain-autodns.test"
	const resourceName = "stalwart_domain.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainConfigAutoDNS(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "dns_management", "Automatic"),
					resource.TestCheckResourceAttrPair(resourceName, "dns_server_id", "stalwart_dns_server.test", "id"),
					resource.TestCheckResourceAttr(resourceName, "publish_records.#", "4"),
					resource.TestCheckTypeSetElemAttr(resourceName, "publish_records.*", "dkim"),
					resource.TestCheckTypeSetElemAttr(resourceName, "publish_records.*", "spf"),
					resource.TestCheckTypeSetElemAttr(resourceName, "publish_records.*", "mx"),
					resource.TestCheckTypeSetElemAttr(resourceName, "publish_records.*", "dmarc"),
					// Independent server-side verification of the union and its set.
					checkServerDomain(c, resourceName, func(d client.Domain) error {
						return firstErr(
							wantVariant("dnsManagement", d.DNSManagement, "Automatic"),
							wantPublishRecords(d.DNSManagement, "dkim", "spf", "mx", "dmarc"),
						)
					}),
				),
			},
			// Import and verify the publish_records set round-trips from a real
			// Domain/get response (the path that originally failed to unmarshal).
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           name,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dns_zone_file"},
			},
		},
	})
}

// wantPublishRecords asserts the dnsManagement union is Automatic and its
// publishRecords set contains exactly want.
func wantPublishRecords(ref *client.TypedRef, want ...string) error {
	if ref == nil {
		return fmt.Errorf("dnsManagement: got nil, want Automatic with publishRecords")
	}
	return wantSetPtr("dnsManagement.publishRecords", ref.PublishRecords, want...)
}

func testAccDomainConfigAutoDNS(name string) string {
	// A local Tsig DNS server validates only the key format and makes no
	// outbound calls, so it is safe to reference from an Automatic-DNS domain in
	// the sandboxed test environment.
	return fmt.Sprintf(`
resource "stalwart_dns_server" "test" {
  type           = "Tsig"
  description    = "tf-acc-autodns"
  host           = "127.0.0.1"
  key_name       = "test.key."
  key            = "dGYtYWNjLXRzaWctdGVzdC1rZXktMzItYnl0ZXMhIQ=="
  protocol       = "udp"
  tsig_algorithm = "hmac-sha256"
}

resource "stalwart_domain" "test" {
  name            = %[1]q
  dns_management  = "Automatic"
  dns_server_id   = stalwart_dns_server.test.id
  publish_records = ["dkim", "spf", "mx", "dmarc"]
}
`, name)
}

func testAccDomainConfigFull(name string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name           = %[1]q
  description    = "initial description"
  catchall       = "postmaster@%[1]s"
  aliases        = ["alias1.%[1]s", "alias2.%[1]s"]
  enabled        = false
  subaddressing  = "Disabled"
  allow_relaying = true
  report_address = "mailto:dmarc@%[1]s"
  dkim_management = "Manual"
}
`, name)
}

func testAccDomainConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name           = %[1]q
  description    = "updated description"
  catchall       = "hostmaster@%[1]s"
  aliases        = ["alias3.%[1]s"]
  enabled        = true
  subaddressing  = "Disabled"
  allow_relaying = false
  report_address = "mailto:dmarc@%[1]s"
  dkim_management = "Manual"
}
`, name)
}
