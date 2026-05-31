// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

//go:build tools

// Package tools pins build-time tool dependencies so that `make generate` uses
// a consistent version of tfplugindocs. See
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	// Documentation generation for the Terraform Registry.
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name stalwart
