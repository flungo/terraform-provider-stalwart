// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// TestAccDkimSignatureResource exercises a stalwart_dkim_signature through its
// full lifecycle and verifies every writable field in state and on the server.
// DKIM signatures import by opaque id (ULID), so the import step reads the id
// from state via ImportStateIdFunc.
func TestAccDkimSignatureResource(t *testing.T) {
	c := accClient(t)
	const domain = "tf-acc-dkim.test"
	const resourceName = "stalwart_dkim_signature.test"
	privateKey := generateEd25519PEM(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDkimConfig(domain, "tfsel", privateKey,
					"simple/simple", []string{"From", "To", "Subject"}, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "selector", "tfsel"),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "ed25519-sha256"),
					resource.TestCheckResourceAttr(resourceName, "canonicalization", "simple/simple"),
					resource.TestCheckResourceAttr(resourceName, "report", "false"),
					resource.TestCheckResourceAttr(resourceName, "headers.#", "3"),
					resource.TestCheckResourceAttrSet(resourceName, "public_key"),
					checkServerDkim(c, resourceName, func(sig client.DkimSignature) error {
						return firstErr(
							wantStr("@type", sig.Type, "Dkim1Ed25519Sha256"),
							wantStr("selector", sig.Selector, "tfsel"),
							wantStr("canonicalization", sig.Canonicalization, "simple/simple"),
							wantBool("report", sig.Report, false),
							wantSet("headers", sig.Headers, "From", "To", "Subject"),
						)
					}),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateIdFunc:       importIDFromState(resourceName),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"private_key", "domain"},
			},
			// Update the mutable canonicalization and report fields.
			{
				Config: testAccDkimConfig(domain, "tfsel", privateKey,
					"relaxed/relaxed", []string{"From", "To", "Subject"}, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "canonicalization", "relaxed/relaxed"),
					resource.TestCheckResourceAttr(resourceName, "report", "true"),
					checkServerDkim(c, resourceName, func(sig client.DkimSignature) error {
						return firstErr(
							wantStr("canonicalization", sig.Canonicalization, "relaxed/relaxed"),
							wantBool("report", sig.Report, true),
						)
					}),
				),
			},
		},
	})
}

// generateEd25519PEM produces a fresh PKCS#8 PEM-encoded Ed25519 private key for
// use as a DKIM signing key, so the test embeds no static secret.
func generateEd25519PEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating ed25519 key: %s", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshalling key: %s", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// importIDFromState returns an ImportStateIdFunc that reads the resource's id
// attribute from state (used for resources imported by opaque id).
func importIDFromState(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		return resourceID(s, resourceName)
	}
}

func testAccDkimConfig(domain, selector, privateKey, canon string, headers []string, report bool) string {
	quotedHeaders := ""
	for i, h := range headers {
		if i > 0 {
			quotedHeaders += ", "
		}
		quotedHeaders += fmt.Sprintf("%q", h)
	}
	return fmt.Sprintf(`
resource "stalwart_domain" "test" {
  name = %[1]q
}

resource "stalwart_dkim_signature" "test" {
  domain_id        = stalwart_domain.test.id
  selector         = %[2]q
  algorithm        = "ed25519-sha256"
  private_key      = %[3]q
  canonicalization = %[4]q
  headers          = [%[5]s]
  report           = %[6]t
}
`, domain, selector, strings.TrimSpace(privateKey)+"\n", canon, quotedHeaders, report)
}
