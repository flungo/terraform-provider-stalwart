// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// This file holds shared acceptance-test helpers. It deliberately keeps the
// verification path independent of the provider's own read/state code: tests
// assert server state through a direct JMAP client built from the same
// environment the provider uses, so a bug in the provider's read logic cannot
// mask a bug in its write logic (and vice versa).

// accClient builds a JMAP client straight from the STALWART_* environment that
// TestMain populated, bypassing the provider entirely. Used by checks that
// independently verify what was actually persisted on the server.
//
// Like the resource.Test harness, it is a no-op path when acceptance testing is
// disabled: if TF_ACC is unset the test is skipped before any client is built,
// so calling it at the top of a test function is safe.
func accClient(t *testing.T) *client.Client {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC is set")
	}
	c, err := client.New(client.Config{
		Endpoint: os.Getenv("STALWART_ENDPOINT"),
		Token:    os.Getenv("STALWART_TOKEN"),
		Username: os.Getenv("STALWART_USERNAME"),
		Password: os.Getenv("STALWART_PASSWORD"),
	})
	if err != nil {
		t.Fatalf("building verification client: %s", err)
	}
	return c
}

// accCtx returns a context for verification calls.
func accCtx() context.Context { return context.Background() }

// hclStringList renders a Go slice as an HCL list literal, e.g. `["a", "b"]`.
func hclStringList(items []string) string {
	out := "["
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", s)
	}
	return out + "]"
}

// accServerVersion returns the schema generation of the server under test, so
// a test can pick the attributes and wire names that version has.
func accServerVersion(t *testing.T) client.ServerVersion {
	t.Helper()
	v, err := accClient(t).ServerVersion(accCtx())
	if err != nil {
		t.Fatalf("detecting server version: %s", err)
	}
	return v
}

// accRawClient builds a verification client that never translates property
// names: pinning it to v1 makes the v0.16 rename table a no-op, so a get
// returns exactly the keys the server sent.
func accRawClient(t *testing.T) *client.Client {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC is set")
	}
	c, err := client.New(client.Config{
		Endpoint:      os.Getenv("STALWART_ENDPOINT"),
		Token:         os.Getenv("STALWART_TOKEN"),
		Username:      os.Getenv("STALWART_USERNAME"),
		Password:      os.Getenv("STALWART_PASSWORD"),
		ServerVersion: client.Version1,
	})
	if err != nil {
		t.Fatalf("building raw verification client: %s", err)
	}
	return c
}
