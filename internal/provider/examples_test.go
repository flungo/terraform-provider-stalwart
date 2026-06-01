// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// These tests apply the Terraform files under examples/ against the throwaway
// Stalwart server, so that regressions or nonsense in the published examples are
// caught and the examples get a genuine end-to-end exercise (real
// init/plan/apply/destroy through Terraform).
//
// Most per-resource example files are documentation fragments that reference a
// domain (and sometimes variables) defined elsewhere. Each test supplies the
// minimal fixtures the fragment needs to become applyable. Data-source examples
// read an object by literal name, so they run in two steps: create the fixture,
// then read it.

// examplesDir is the examples/ directory relative to this package.
const examplesDir = "../../examples"

// strongPassword is an uncommon multi-word passphrase that passes Stalwart's
// zxcvbn strength check.
const strongPassword = "correct-horse-battery-staple-42"

// terraformBlock matches a top-level `terraform { ... }` block (the examples'
// required_providers boilerplate), which the acceptance-test framework supplies
// itself via ProtoV6ProviderFactories.
var terraformBlock = regexp.MustCompile(`(?ms)^terraform\s*\{.*?^\}\s*`)

// readExample reads an example file relative to examplesDir and strips its
// `terraform {}` block so the config is driven by the in-process provider.
func readExample(t *testing.T, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir, relPath))
	if err != nil {
		t.Fatalf("reading example %s: %s", relPath, err)
	}
	return terraformBlock.ReplaceAllString(string(data), "")
}

// domainFixture is the prerequisite domain that fragment examples reference as
// stalwart_domain.example.
const domainFixture = `
resource "stalwart_domain" "example" {
  name = "example.com"
}
`

// TestAccExampleMain applies the complete examples/main.tf end-to-end: every
// resource type plus a data source, in one dependency-correct configuration.
func TestAccExampleMain(t *testing.T) {
	c := accClient(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: readExample(t, "main.tf"),
				ConfigVariables: config.Variables{
					"dkim_private_key": config.StringVariable(generateEd25519PEM(t)),
					"alice_password":   config.StringVariable(strongPassword),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stalwart_domain.example", "name", "example.com"),
					resource.TestCheckResourceAttrSet("stalwart_dkim_signature.example", "id"),
					resource.TestCheckResourceAttrSet("stalwart_role.support", "id"),
					resource.TestCheckResourceAttrSet("stalwart_group.team", "id"),
					resource.TestCheckResourceAttr("stalwart_account.alice", "email_address", "alice@example.com"),
					resource.TestCheckResourceAttrSet("stalwart_mailing_list.announce", "id"),
					resource.TestCheckResourceAttrSet("data.stalwart_dns_records.example", "zone_file"),
					// Independent server-side confirmation that the apply landed.
					checkServerDomain(c, "stalwart_domain.example", func(d client.Domain) error {
						return wantStr("name", d.Name, "example.com")
					}),
				),
			},
		},
	})
}

// TestAccExampleResources applies each per-resource example fragment with the
// fixtures it needs.
func TestAccExampleResources(t *testing.T) {
	cases := map[string]struct {
		file    string
		fixture string
		vars    config.Variables
		checkID string // a resource to assert got an id
	}{
		"domain": {
			file:    "resources/stalwart_domain/resource.tf",
			checkID: "stalwart_domain.example",
		},
		"role": {
			file:    "resources/stalwart_role/resource.tf",
			checkID: "stalwart_role.support",
		},
		"account": {
			file:    "resources/stalwart_account/resource.tf",
			fixture: domainFixture + "\nvariable \"alice_password\" { type = string }\n",
			vars:    config.Variables{"alice_password": config.StringVariable(strongPassword)},
			checkID: "stalwart_account.alice",
		},
		"group": {
			file:    "resources/stalwart_group/resource.tf",
			fixture: domainFixture,
			checkID: "stalwart_group.team",
		},
		"mailing_list": {
			file:    "resources/stalwart_mailing_list/resource.tf",
			fixture: domainFixture,
			checkID: "stalwart_mailing_list.announce",
		},
		"dkim_signature": {
			file:    "resources/stalwart_dkim_signature/resource.tf",
			fixture: domainFixture + "\nvariable \"dkim_private_key\" { type = string }\n",
			checkID: "stalwart_dkim_signature.example",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_ = accClient(t) // skip unless TF_ACC
			vars := tc.vars
			if name == "dkim_signature" {
				// Generate the signing key per run so no secret is embedded.
				vars = config.Variables{"dkim_private_key": config.StringVariable(generateEd25519PEM(t))}
			}
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:          tc.fixture + readExample(t, tc.file),
						ConfigVariables: vars,
						Check:           resource.TestCheckResourceAttrSet(tc.checkID, "id"),
					},
				},
			})
		})
	}
}

// TestAccExampleDataSources applies each data-source example. Data sources read
// an object by literal name, so the fixture is created in a first step and the
// example data source reads it in a second (the fixture persists across steps).
func TestAccExampleDataSources(t *testing.T) {
	cases := map[string]struct {
		file    string
		fixture string
		check   resource.TestCheckFunc
	}{
		"domain": {
			file:    "data-sources/stalwart_domain/data-source.tf",
			fixture: domainFixture,
			check:   resource.TestCheckResourceAttr("data.stalwart_domain.example", "name", "example.com"),
		},
		"dns_records": {
			file:    "data-sources/stalwart_dns_records/data-source.tf",
			fixture: domainFixture,
			check:   resource.TestCheckResourceAttrSet("data.stalwart_dns_records.example", "zone_file"),
		},
		"account": {
			file: "data-sources/stalwart_account/data-source.tf",
			fixture: domainFixture + `
resource "stalwart_account" "alice" {
  domain_id = stalwart_domain.example.id
  name      = "alice"
  password  = "` + strongPassword + `"
}
`,
			check: resource.TestCheckResourceAttr("data.stalwart_account.alice", "email_address", "alice@example.com"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_ = accClient(t) // skip unless TF_ACC
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					// Step 1: create the fixture object.
					{Config: tc.fixture},
					// Step 2: keep the fixture and read it via the example.
					{
						Config: tc.fixture + readExample(t, tc.file),
						Check:  tc.check,
					},
				},
			})
		})
	}
}
