// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &acmeProviderResource{}
	_ resource.ResourceWithConfigure   = &acmeProviderResource{}
	_ resource.ResourceWithImportState = &acmeProviderResource{}
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
				Optional:      true,
				Computed:      true,
				Description:   "Maximum number of ACME challenge retries before giving up.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
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

func (r *acmeProviderResource) toAPI(ctx context.Context, m *acmeProviderResourceModel, diags *fwDiags) *client.AcmeProvider {
	p := &client.AcmeProvider{
		ChallengeType: strPtr(m.ChallengeType),
		Directory:     strPtr(m.Directory),
	}
	p.Contact = stringSetPtr(stringSetSlice(ctx, m.Contact, diags))
	if s := strPtr(m.RenewBefore); s != nil {
		p.RenewBefore = s
	}
	if !m.MaxRetries.IsNull() && !m.MaxRetries.IsUnknown() {
		v := m.MaxRetries.ValueInt64()
		p.MaxRetries = &v
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

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
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

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
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
