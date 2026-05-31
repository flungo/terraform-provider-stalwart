// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories instantiates the provider during acceptance
// testing. The factory is invoked once per test case to obtain a provider
// server with the in-process provider under test.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"stalwart": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that the environment is configured for acceptance
// testing before any acceptance test runs. Acceptance tests talk to a real
// Stalwart instance and only run when TF_ACC is set.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if v := os.Getenv("STALWART_ENDPOINT"); v == "" {
		t.Fatal("STALWART_ENDPOINT must be set for acceptance tests")
	}
	if os.Getenv("STALWART_TOKEN") == "" &&
		(os.Getenv("STALWART_USERNAME") == "" || os.Getenv("STALWART_PASSWORD") == "") {
		t.Fatal("STALWART_TOKEN (or STALWART_USERNAME and STALWART_PASSWORD) must be set for acceptance tests")
	}
}
