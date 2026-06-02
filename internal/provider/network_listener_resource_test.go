// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccNetworkListenerResource exercises the full lifecycle of a
// stalwart_network_listener resource (create, read, update, import, delete)
// and verifies fields via both Terraform state and direct JMAP client.
func TestAccNetworkListenerResource(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_network_listener.test"
	// Use high-numbered ports to avoid conflicts with other listeners.
	const bindAddr = "0.0.0.0:12025"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with all fields set.
			{
				Config: testAccNetworkListenerConfigFull(bindAddr),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-smtp"),
					resource.TestCheckResourceAttr(resourceName, "protocol", "smtp"),
					resource.TestCheckResourceAttr(resourceName, "tls_implicit", "false"),
					resource.TestCheckResourceAttr(resourceName, "bind.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "bind.*", bindAddr),
					resource.TestCheckResourceAttr(resourceName, "override_proxy_trusted_networks.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "override_proxy_trusted_networks.*", "10.10.10.0/24"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					checkServerNetworkListener(c, resourceName, func(l client.NetworkListener) error {
						return firstErr(
							wantStr("name", l.Name, "tf-acc-smtp"),
							wantStr("protocol", l.Protocol, "smtp"),
							wantBool("tlsImplicit", l.TLSImplicit, false),
							wantSet("bind", l.Bind, bindAddr),
							wantSet("overrideProxyTrustedNetworks", l.OverrideProxyTrustedNetworks, "10.10.10.0/24"),
						)
					}),
				),
			},
			// Import by name.
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     "tf-acc-smtp",
				ImportStateVerify: true,
			},
			// Update mutable fields.
			{
				Config: testAccNetworkListenerConfigUpdated(bindAddr),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "override_proxy_trusted_networks.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "override_proxy_trusted_networks.*", "10.10.10.0/24"),
					resource.TestCheckTypeSetElemAttr(resourceName, "override_proxy_trusted_networks.*", "192.168.1.0/24"),
					checkServerNetworkListener(c, resourceName, func(l client.NetworkListener) error {
						return wantSet("overrideProxyTrustedNetworks", l.OverrideProxyTrustedNetworks,
							"10.10.10.0/24", "192.168.1.0/24")
					}),
				),
			},
		},
	})
}

// TestAccNetworkListenerResourceImplicitTLS verifies an SMTPS listener with
// implicit TLS enabled.
func TestAccNetworkListenerResourceImplicitTLS(t *testing.T) {
	c := accClient(t)
	const resourceName = "stalwart_network_listener.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "stalwart_network_listener" "test" {
  name         = "tf-acc-smtps"
  bind         = ["0.0.0.0:12465"]
  protocol     = "smtp"
  tls_implicit = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "tls_implicit", "true"),
					resource.TestCheckResourceAttr(resourceName, "bind.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "bind.*", "0.0.0.0:12465"),
					checkServerNetworkListener(c, resourceName, func(l client.NetworkListener) error {
						return firstErr(
							wantStr("name", l.Name, "tf-acc-smtps"),
							wantBool("tlsImplicit", l.TLSImplicit, true),
							wantSet("bind", l.Bind, "0.0.0.0:12465"),
						)
					}),
				),
			},
		},
	})
}

func testAccNetworkListenerConfigFull(bindAddr string) string {
	return fmt.Sprintf(`
resource "stalwart_network_listener" "test" {
  name         = "tf-acc-smtp"
  bind         = [%q]
  protocol     = "smtp"
  tls_implicit = false
  override_proxy_trusted_networks = ["10.10.10.0/24"]
}
`, bindAddr)
}

func testAccNetworkListenerConfigUpdated(bindAddr string) string {
	return fmt.Sprintf(`
resource "stalwart_network_listener" "test" {
  name         = "tf-acc-smtp"
  bind         = [%q]
  protocol     = "smtp"
  tls_implicit = false
  override_proxy_trusted_networks = ["10.10.10.0/24", "192.168.1.0/24"]
}
`, bindAddr)
}
