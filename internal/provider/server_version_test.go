// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// imageVersion matches the tag of a stalwartlabs/stalwart image, e.g. v0.16.27.
var imageVersion = regexp.MustCompile(`:v?(0\.16|1)(\.[0-9]+)*$`)

// TestAccServerVersionDetection checks that version detection agrees with the
// image tag the CI matrix started, so a detection bug cannot hide behind a
// suite that silently runs everything against one dialect.
func TestAccServerVersionDetection(t *testing.T) {
	got := accServerVersion(t)
	img := os.Getenv("STALWART_TEST_IMAGE")
	m := imageVersion.FindStringSubmatch(img)
	if m == nil {
		t.Skipf("cannot infer the expected version from STALWART_TEST_IMAGE=%q", img)
	}
	want, err := client.ParseServerVersion(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("detected Stalwart %v, but the image under test is %s", got, img)
	}
}

// TestAccWireNames checks the property names that actually reach the server:
// the domain's enable flag must be stored under the name the server's version
// uses. The typed checks elsewhere read through the client's rename table, so
// this is the one assertion that does not depend on it.
func TestAccWireNames(t *testing.T) {
	raw := accRawClient(t)
	wantKey, otherKey := "enable", "isEnabled"
	if accServerVersion(t) == client.Version016 {
		wantKey, otherKey = otherKey, wantKey
	}
	const resourceName = "stalwart_domain.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
resource "stalwart_domain" "test" {
  name    = "wire-names.test"
  enabled = false
}
`,
			Check: func(s *terraform.State) error {
				id, err := resourceID(s, resourceName)
				if err != nil {
					return err
				}
				resp, err := raw.Get(accCtx(), client.TypeDomain, []string{id})
				if err != nil {
					return err
				}
				if len(resp.List) != 1 {
					return fmt.Errorf("domain %s not found", id)
				}
				var obj map[string]json.RawMessage
				if err := json.Unmarshal(resp.List[0], &obj); err != nil {
					return err
				}
				if string(obj[wantKey]) != "false" {
					return fmt.Errorf("server property %q = %s, want false (object: %s)", wantKey, obj[wantKey], resp.List[0])
				}
				if _, ok := obj[otherKey]; ok {
					return fmt.Errorf("server returned property %q, which this version should not have", otherKey)
				}
				return nil
			},
		}},
	})
}

// TestAccVersionedAttributeRejected checks that an attribute the server's
// version does not have fails at plan time with an actionable error, rather
// than reaching the server or producing a perpetual diff.
func TestAccVersionedAttributeRejected(t *testing.T) {
	config := `
resource "stalwart_acme_provider" "test" {
  challenge_type = "Dns01"
  directory      = "https://acme-staging-v02.api.letsencrypt.org/directory"
  contact        = ["mailto:acme@stalwart-tf-acc.net"]
  max_retries    = 3
}
`
	if accServerVersion(t) == client.Version016 {
		config = `
resource "stalwart_acme_provider" "test" {
  challenge_type = "Dns01"
  directory      = "https://acme-staging-v02.api.letsencrypt.org/directory"
  contact        = ["mailto:acme@stalwart-tf-acc.net"]
  description    = "not settable on v0.16"
}
`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      config,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Attribute not supported by this Stalwart version`),
		}},
	})
}
