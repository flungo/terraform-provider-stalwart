// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// domainIDAttribute returns the shared `domain_id` resource attribute. It is
// Optional+Computed so that practitioners may instead reference a domain by
// name via the companion `domain` attribute, in which case the id is resolved
// and stored automatically. Exactly one of `domain_id` or `domain` must be set.
func domainIDAttribute() rschema.StringAttribute {
	return rschema.StringAttribute{
		Optional: true,
		Computed: true,
		Description: "Opaque id of the domain this object belongs to. " +
			"Mutually exclusive with `domain`.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
			stringplanmodifier.UseStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.ExactlyOneOf(
				path.MatchRoot("domain_id"),
				path.MatchRoot("domain"),
			),
		},
	}
}

// domainNameAttribute returns the shared `domain` resource attribute, an
// alternative to `domain_id` that references the domain by name.
func domainNameAttribute() rschema.StringAttribute {
	return rschema.StringAttribute{
		Optional: true,
		Description: "Name of the domain this object belongs to, e.g. `example.com`. " +
			"Resolved to a domain id at apply time. Mutually exclusive with `domain_id`.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
}

// fwDiags is a short alias for the framework diagnostics type, used by the
// per-resource conversion helpers.
type fwDiags = diag.Diagnostics

// deref returns the value pointed to by p, or the zero value when p is nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// configureClient extracts the *client.Client placed on the provider data by
// Configure. It is shared by every resource and data source. providerData is
// nil during the initial provider configuration walk, which is not an error.
func configureClient(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", providerData),
		)
		return nil
	}
	return c
}

// resolveDomainID resolves a domain reference expressed either as an explicit
// opaque id (domainID) or as a domain name (domainName). Exactly one of the two
// must be set; this is enforced at schema level by the caller's validators.
func resolveDomainID(ctx context.Context, c *client.Client, domainID, domainName types.String) (string, error) {
	switch {
	case !domainID.IsNull() && domainID.ValueString() != "":
		return domainID.ValueString(), nil
	case !domainName.IsNull() && domainName.ValueString() != "":
		return resolveByNameOrID(ctx, c, client.TypeDomain, domainName.ValueString(),
			map[string]any{"name": domainName.ValueString()})
	default:
		return "", errors.New("one of `domain_id` or `domain` must be set")
	}
}

// resolveByNameOrID returns an object id from a reference string. If ref is a
// an object id it is treated as the object id directly; otherwise it is resolved to an
// id with the supplied query filter (a name lookup).
func resolveByNameOrID(ctx context.Context, c *client.Client, objType, ref string, filter any) (string, error) {
	if client.IsID(ref) {
		return ref, nil
	}
	id, err := c.QueryOne(ctx, objType, filter)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return "", fmt.Errorf("no %s found matching %q", objType, ref)
		}
		return "", err
	}
	return id, nil
}

// splitEmail splits an email address into its local part and domain. It returns
// ok=false when the input is not of the form "local@domain".
func splitEmail(addr string) (local, domain string, ok bool) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == '@' {
			if i == 0 || i == len(addr)-1 {
				return "", "", false
			}
			return addr[:i], addr[i+1:], true
		}
	}
	return "", "", false
}

// resolveAccountByEmailOrID resolves an Account (user or group) import id. A
// an object id is treated as the object id directly; otherwise the reference must be an
// email address ("local@domain") which is resolved to an id via the domain and
// account name filters.
func resolveAccountByEmailOrID(ctx context.Context, c *client.Client, ref string) (string, error) {
	if client.IsID(ref) {
		return ref, nil
	}
	local, domain, ok := splitEmail(ref)
	if !ok {
		return "", fmt.Errorf("import id %q must be an object id or an email address (local@domain)", ref)
	}
	domainID, err := c.QueryOne(ctx, client.TypeDomain, map[string]any{"name": domain})
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return "", fmt.Errorf("no domain found matching %q", domain)
		}
		return "", err
	}
	id, err := c.QueryOne(ctx, client.TypeAccount, map[string]any{"name": local, "domainId": domainID})
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return "", fmt.Errorf("no account found matching %q", ref)
		}
		return "", err
	}
	return id, nil
}

// --- pointer / framework type conversion helpers -----------------------------

// strPtr returns a pointer to the string value of a types.String, or nil when
// the value is null or unknown.
func strPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// strValue converts a *string to a types.String, mapping nil to null.
func strValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// boolPtr returns a pointer to the bool value of a types.Bool, or nil when the
// value is null or unknown.
func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// boolValue converts a *bool to a types.Bool, mapping nil to null.
func boolValue(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

// int64Value converts a *int64 to a types.Int64, mapping nil to null.
func int64Value(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// stringSetSlice converts a Terraform set of strings to a Go slice. A null or
// unknown set yields a nil slice. Used for collection properties the server
// stores unordered (Stalwart `Map<T>`), which are modelled as `types.Set` so
// that config order is not significant.
func stringSetSlice(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

// stringListValue converts a Go slice to a Terraform list of strings, mapping a
// nil slice to a null list.
func stringListValue(slice []string) (types.List, diag.Diagnostics) {
	if slice == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(context.Background(), types.StringType, slice)
}

// stringSetValue converts a Go slice to a Terraform set of strings, mapping a
// nil slice to a null set.
func stringSetValue(slice []string) (types.Set, diag.Diagnostics) {
	if slice == nil {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(context.Background(), types.StringType, slice)
}

// stringSetPtr returns a pointer to a client.StringSet built from s, always
// non-nil so the field is present on the wire (Stalwart requires collection
// properties to be sent as an object, using {} for "no items"). A nil slice
// yields a pointer to an empty set.
func stringSetPtr(s []string) *client.StringSet {
	set := client.StringSet(s)
	return &set
}
