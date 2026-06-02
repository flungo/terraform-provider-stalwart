// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccDnsServerResourceTsig exercises the full lifecycle of a stalwart_dns_server
// resource with type=Tsig (create, read, update, import, delete) and verifies
// fields both via Terraform state and direct JMAP client.
//
// TSIG is used because it validates only the base64 key format locally, without
// making any outbound API calls to a DNS provider — making it suitable for the
// sandboxed test environment.
func TestAccDnsServerResourceTsig(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_dns_server.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with all fields set.
			{
				Config: testAccDnsServerTsigConfigFull(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "type", "Tsig"),
					resource.TestCheckResourceAttr(resourceName, "description", "tf-acc-tsig"),
					resource.TestCheckResourceAttr(resourceName, "host", "127.0.0.1"),
					resource.TestCheckResourceAttr(resourceName, "port", "53"),
					resource.TestCheckResourceAttr(resourceName, "key_name", "test.key."),
					resource.TestCheckResourceAttr(resourceName, "protocol", "udp"),
					resource.TestCheckResourceAttr(resourceName, "tsig_algorithm", "hmac-sha256"),
					resource.TestCheckResourceAttr(resourceName, "timeout", "30s"),
					resource.TestCheckResourceAttr(resourceName, "ttl", "5m"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "10s"),
					resource.TestCheckResourceAttr(resourceName, "propagation_timeout", "2m"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					checkServerDnsServer(c, resourceName, func(srv client.DnsServer) error {
						return firstErr(
							wantStr("@type", srv.Type, "Tsig"),
							wantStr("description", srv.Description, "tf-acc-tsig"),
							wantStr("host", srv.Host, "127.0.0.1"),
							wantStr("keyName", srv.KeyName, "test.key."),
							wantStr("protocol", srv.Protocol, "udp"),
							wantStr("tsigAlgorithm", srv.TsigAlgorithm, "hmac-sha256"),
						)
					}),
				),
			},
			// Import by id.
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
			// Update mutable fields. Duration fields omitted from config are preserved
			// from prior state (Optional+Computed with UseStateForUnknown).
			{
				Config: testAccDnsServerTsigConfigUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "tf-acc-tsig-updated"),
					resource.TestCheckResourceAttr(resourceName, "timeout", "1m"),
					resource.TestCheckResourceAttr(resourceName, "ttl", "5m"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "10s"),
					resource.TestCheckResourceAttr(resourceName, "propagation_timeout", "2m"),
					checkServerDnsServer(c, resourceName, func(srv client.DnsServer) error {
						return wantStr("description", srv.Description, "tf-acc-tsig-updated")
					}),
				),
			},
		},
	})
}

func testAccDnsServerTsigConfigFull() string {
	// key is base64("tf-acc-tsig-test-key-32-bytes!!") — 32 bytes, valid base64.
	return `
resource "stalwart_dns_server" "test" {
  type                = "Tsig"
  description         = "tf-acc-tsig"
  host                = "127.0.0.1"
  port                = 53
  key_name            = "test.key."
  key                 = "dGYtYWNjLXRzaWctdGVzdC1rZXktMzItYnl0ZXMhIQ=="
  protocol            = "udp"
  tsig_algorithm      = "hmac-sha256"
  timeout             = "30s"
  ttl                 = "5m"
  polling_interval    = "10s"
  propagation_timeout = "2m"
}
`
}

func testAccDnsServerTsigConfigUpdated() string {
	return `
resource "stalwart_dns_server" "test" {
  type           = "Tsig"
  description    = "tf-acc-tsig-updated"
  host           = "127.0.0.1"
  key_name       = "test.key."
  key            = "dGYtYWNjLXRzaWctdGVzdC1rZXktMzItYnl0ZXMhIQ=="
  protocol       = "udp"
  tsig_algorithm = "hmac-sha256"
  timeout        = "1m"
}
`
}
