// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/flungo/terraform-provider-stalwart/internal/acctest"
)

// testAccProtoV6ProviderFactories instantiates the provider during acceptance
// testing. The factory is invoked once per test case to obtain a provider
// server with the in-process provider under test.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"stalwart": providerserver.NewProtocol6WithError(New("test")()),
}

// TestMain manages the lifecycle of the disposable Stalwart test instance.
//
// When acceptance tests are enabled (TF_ACC set) and the caller has not pointed
// the suite at an externally-managed server (STALWART_ENDPOINT unset), it boots
// a throwaway Stalwart container, exports the credentials the provider reads,
// runs the tests, and tears the container down. This is what lets `make testacc`
// work with no user-provided instance or environment variables.
//
// If STALWART_ENDPOINT is already set, the harness steps aside and the tests run
// against that instance instead. If TF_ACC is unset, no acceptance tests run and
// no container is started.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	if os.Getenv("TF_ACC") == "" || os.Getenv("STALWART_ENDPOINT") != "" {
		return m.Run()
	}

	ctx := context.Background()
	container, err := acctest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acctest: unable to start Stalwart container: %v\n", err)
		return 1
	}
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "acctest: unable to terminate container: %v\n", err)
		}
	}()

	// Point the provider (and the per-test pre-check) at the container using the
	// pinned recovery-admin credentials over HTTP Basic auth.
	setEnv("STALWART_ENDPOINT", container.Endpoint())
	setEnv("STALWART_USERNAME", container.AdminUser())
	setEnv("STALWART_PASSWORD", container.AdminPassword())

	code := m.Run()
	if code != 0 {
		// On failure, persist the server's own logs alongside the test output so
		// server-side rejections during CRUD are diagnosable (the container is
		// about to be torn down).
		container.DumpLogs(context.Background())
	}
	return code
}

func setEnv(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		panic(fmt.Sprintf("acctest: setting %s: %v", key, err))
	}
}

// testAccPreCheck validates that the environment is configured for acceptance
// testing before any acceptance test runs. Acceptance tests talk to a real
// Stalwart instance and only run when TF_ACC is set. When the in-process harness
// is used, TestMain has already populated these variables.
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
