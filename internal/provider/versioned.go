// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// versionedAttr is a resource attribute that exists on only one Stalwart
// schema generation. See docs/decisions/004-dual-server-version-support.md.
type versionedAttr struct {
	// name is the Terraform attribute name.
	name string
	// value is the attribute's value in the configuration.
	value attr.Value
	// only is the generation that supports the attribute.
	only client.ServerVersion
	// hint is appended to the error, e.g. what replaced the attribute.
	hint string
}

// serverVersion returns the client's server generation, recording a
// diagnostic and returning VersionUnknown when it cannot be determined.
func serverVersion(ctx context.Context, c *client.Client, diags *diag.Diagnostics) client.ServerVersion {
	v, err := c.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Unable to determine the Stalwart server version", err.Error())
		return client.VersionUnknown
	}
	return v
}

// checkVersionedAttrs reports an attribute error for every attribute that is
// set in the configuration but not supported by the server's schema
// generation. It contacts the server only when at least one such attribute is
// set, so configurations that use only common attributes never trigger
// version detection from a plan. Unknown values are skipped here; the apply
// path re-checks them once known.
func checkVersionedAttrs(ctx context.Context, c *client.Client, attrs []versionedAttr, diags *diag.Diagnostics) {
	if c == nil {
		return
	}
	var version client.ServerVersion
	for _, a := range attrs {
		if a.value == nil || a.value.IsNull() || a.value.IsUnknown() {
			continue
		}
		if version == client.VersionUnknown {
			if version = serverVersion(ctx, c, diags); version == client.VersionUnknown {
				return
			}
		}
		if version != a.only {
			detail := fmt.Sprintf("`%s` is only supported by Stalwart %s servers; this server is Stalwart %s. "+
				"Remove it from the configuration.", a.name, versionLabel(a.only), versionLabel(version))
			if a.hint != "" {
				detail += " " + a.hint
			}
			diags.AddAttributeError(path.Root(a.name), "Attribute not supported by this Stalwart version", detail)
		}
	}
}

// versionLabel renders a generation for humans, e.g. "v0.16".
func versionLabel(v client.ServerVersion) string {
	switch v {
	case client.Version016:
		return "v0.16"
	case client.Version1:
		return "v1"
	default:
		return "(unknown version)"
	}
}
