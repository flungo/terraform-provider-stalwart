// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/flungo/terraform-provider-stalwart/internal/provider"
)

// These are set at build/release time via -ldflags by GoReleaser.
var (
	version = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// The registry address. Must match the published provider address so
		// that Terraform can resolve the provider in configuration.
		Address: "registry.terraform.io/flungo/stalwart",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
