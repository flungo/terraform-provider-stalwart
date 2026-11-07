// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &acmeProviderResource{}
	_ resource.ResourceWithConfigure   = &acmeProviderResource{}
	_ resource.ResourceWithImportState = &acmeProviderResource{}
	_ resource.ResourceWithModifyPlan  = &acmeProviderResource{}
)

// NewAcmeProviderResource is the constructor referenced by the provider.
func NewAcmeProviderResource() resource.Resource {
	return &acmeProviderResource{}
}

type acmeProviderResource struct {
	client *client.Client
}

type acmeProviderResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ChallengeType types.String `tfsdk:"challenge_type"`
	Contact       types.Set    `tfsdk:"contact"`
	Directory     types.String `tfsdk:"directory"`
	RenewBefore   types.String `tfsdk:"renew_before"`
	MaxRetries    types.Int64  `tfsdk:"max_retries"`
	Description   types.String `tfsdk:"description"`

	// Server-set after ACME account registration.
	AccountKey types.String `tfsdk:"account_key"`
	AccountUri types.String `tfsdk:"account_uri"`
}

func (r *acmeProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acme_provider"
}

func (r *acmeProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *acmeProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart ACME provider for automatic TLS certificate issuance (the `AcmeProvider` JMAP object).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier of the ACME provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"challenge_type": schema.StringAttribute{
				Required: true,
				Description: "ACME challenge method: `TlsAlpn01`, `DnsPersist01`, `Dns01`, or `Http01`. " +
					"Changing it replaces the ACME provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"contact": schema.SetAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Contact email addresses registered with the ACME CA, e.g. `[\"mailto:admin@example.com\"]`.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"directory": schema.StringAttribute{
				Required: true,
				Description: "URL of the ACME CA directory endpoint, e.g. " +
					"`https://acme-v02.api.letsencrypt.org/directory`. " +
					"Changing it replaces the ACME provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"renew_before": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Fraction of the certificate lifetime remaining when renewal is triggered: `R12` (1/2), `R23` (2/3), `R34` (3/4), or `R45` (4/5).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"max_retries": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Maximum number of ACME challenge retries before giving up. " +
					"Stalwart v0.16 only: v1 removed the setting (retries are handled internally), " +
					"so on a v1 server it reads as null and setting it is an error.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Human-readable description of the ACME provider. " +
					"Stalwart v1 requires one; when omitted the provider sets it to the `directory` URL on create. " +
					"On v0.16 the server derives it from the directory URL and ACME account, " +
					"so it is read-only there and setting it is an error.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_key": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "ACME account private key (server-set after registration).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_uri": schema.StringAttribute{
				Computed:      true,
				Description:   "ACME account URI (server-set after registration).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// versionedAttrs lists the attributes that exist on only one server version.
func (m *acmeProviderResourceModel) versionedAttrs() []versionedAttr {
	return []versionedAttr{
		{name: "max_retries", value: m.MaxRetries, only: client.Version016,
			hint: "Stalwart v1 removed the setting; ACME retries are handled internally."},
		{name: "description", value: m.Description, only: client.Version1,
			hint: "Stalwart v0.16 derives the description from the directory URL and ACME account."},
	}
}

// ModifyPlan rejects attributes the server's version does not support.
func (r *acmeProviderResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var cfg acmeProviderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkVersionedAttrs(ctx, r.client, cfg.versionedAttrs(), &resp.Diagnostics)
}

// toAPI builds the wire object from the plan. Version-specific properties are
// sent only to the version that has them; the plan can carry a value from
// state for them (Optional+Computed), which must not reach the other version.
func (r *acmeProviderResource) toAPI(ctx context.Context, m *acmeProviderResourceModel, version client.ServerVersion, diags *fwDiags) *client.AcmeProvider {
	p := &client.AcmeProvider{
		ChallengeType: strPtr(m.ChallengeType),
		Directory:     strPtr(m.Directory),
	}
	p.Contact = stringSetPtr(stringSetSlice(ctx, m.Contact, diags))
	if s := strPtr(m.RenewBefore); s != nil {
		p.RenewBefore = s
	}
	if version == client.Version016 && !m.MaxRetries.IsNull() && !m.MaxRetries.IsUnknown() {
		v := m.MaxRetries.ValueInt64()
		p.MaxRetries = &v
	}
	if version == client.Version1 {
		// v1 rejects an empty description (AcmeProvider.validate in
		// crates/registry/src/schema/structs_impl.rs). On create without one,
		// default to the directory URL, the stem of v0.16's derived value.
		p.Description = strPtr(m.Description)
		if p.Description == nil || *p.Description == "" {
			p.Description = strPtr(m.Directory)
		}
	}
	return p
}

// fromAPI populates the model from the server object.
func (r *acmeProviderResource) fromAPI(m *acmeProviderResourceModel, p *client.AcmeProvider, diags *fwDiags) {
	m.ID = strValue(p.ID)
	m.ChallengeType = strValue(p.ChallengeType)
	m.Directory = strValue(p.Directory)
	m.RenewBefore = strValue(p.RenewBefore)
	if p.MaxRetries != nil {
		m.MaxRetries = types.Int64Value(*p.MaxRetries)
	} else {
		m.MaxRetries = types.Int64Null()
	}
	// v0.16 always returns its derived description; v1 returns the stored one,
	// which defaults to empty.
	m.Description = types.StringValue(deref(p.Description))
	m.AccountKey = strValue(p.AccountKey)
	m.AccountUri = strValue(p.AccountUri)

	contact, d := stringSetValue(deref(p.Contact))
	diags.Append(d...)
	m.Contact = contact
}

func (r *acmeProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan acmeProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.applyBody(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeAcmeProvider, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create ACME provider", err.Error())
		return
	}

	var created client.AcmeProvider
	if err := r.client.GetOne(ctx, client.TypeAcmeProvider, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read ACME provider after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// applyBody re-checks the version-specific attributes against the now-known
// configuration and builds the wire object for create or update.
func (r *acmeProviderResource) applyBody(ctx context.Context, config tfsdk.Config, plan *acmeProviderResourceModel, diags *fwDiags) *client.AcmeProvider {
	var cfg acmeProviderResourceModel
	diags.Append(config.Get(ctx, &cfg)...)
	if diags.HasError() {
		return nil
	}
	checkVersionedAttrs(ctx, r.client, cfg.versionedAttrs(), diags)
	version := serverVersion(ctx, r.client, diags)
	if diags.HasError() {
		return nil
	}
	return r.toAPI(ctx, plan, version, diags)
}

func (r *acmeProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state acmeProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var p client.AcmeProvider
	if err := r.client.GetOne(ctx, client.TypeAcmeProvider, state.ID.ValueString(), &p); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read ACME provider", err.Error())
		return
	}
	r.fromAPI(&state, &p, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *acmeProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan acmeProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state acmeProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.applyBody(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// challenge_type and directory are RequiresReplace; both are immutable on update.
	body.ChallengeType = nil
	body.Directory = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeAcmeProvider, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update ACME provider", err.Error())
		return
	}

	var updated client.AcmeProvider
	if err := r.client.GetOne(ctx, client.TypeAcmeProvider, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read ACME provider after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *acmeProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state acmeProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeAcmeProvider, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete ACME provider", err.Error())
	}
}

// ImportState imports an ACME provider by its opaque id.
func (r *acmeProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !client.IsID(req.ID) {
		resp.Diagnostics.AddError("Invalid ACME provider import id",
			fmt.Sprintf("%q is not a valid object id. ACME providers must be imported by their opaque id.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
