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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &networkListenerResource{}
	_ resource.ResourceWithConfigure   = &networkListenerResource{}
	_ resource.ResourceWithImportState = &networkListenerResource{}
)

// NewNetworkListenerResource is the constructor referenced by the provider.
func NewNetworkListenerResource() resource.Resource {
	return &networkListenerResource{}
}

type networkListenerResource struct {
	client *client.Client
}

type networkListenerResourceModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	Bind                         types.Set    `tfsdk:"bind"`
	Protocol                     types.String `tfsdk:"protocol"`
	TLSImplicit                  types.Bool   `tfsdk:"tls_implicit"`
	OverrideProxyTrustedNetworks types.Set    `tfsdk:"override_proxy_trusted_networks"`
}

func (r *networkListenerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_listener"
}

func (r *networkListenerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *networkListenerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart network listener (the `NetworkListener` JMAP object). " +
			"A listener binds to one or more addresses and handles a specific mail or HTTP protocol.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier of the listener.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Unique name for this listener, e.g. `smtp`, `smtps`, `imaps`. " +
					"Changing the name replaces the listener.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"bind": schema.SetAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Set of `host:port` addresses the listener binds to, e.g. `[\"0.0.0.0:25\"]`.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"protocol": schema.StringAttribute{
				Required: true,
				Description: "Protocol handled by this listener: `smtp`, `lmtp`, `imap`, `pop3`, " +
					"`http`, or `manageSieve`.",
			},
			"tls_implicit": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Whether TLS is negotiated immediately on connection (implicit TLS / SMTPS/IMAPS). " +
					"Set to `true` for port 465 (SMTPS) and port 993 (IMAPS). Defaults to `false`.",
			},
			"override_proxy_trusted_networks": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "CIDR blocks trusted to send proxy-protocol headers for this listener, " +
					"e.g. `[\"10.10.10.0/24\"]`. Overrides the global proxy trusted network list.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *networkListenerResource) toAPI(ctx context.Context, m *networkListenerResourceModel, diags *fwDiags) *client.NetworkListener {
	l := &client.NetworkListener{
		Name:        strPtr(m.Name),
		Protocol:    strPtr(m.Protocol),
		TLSImplicit: boolPtr(m.TLSImplicit),
	}
	l.Bind = stringSetPtr(stringSetSlice(ctx, m.Bind, diags))
	if s := stringSetSlice(ctx, m.OverrideProxyTrustedNetworks, diags); s != nil {
		l.OverrideProxyTrustedNetworks = stringSetPtr(s)
	}
	return l
}

// fromAPI populates the model from the server object.
func (r *networkListenerResource) fromAPI(m *networkListenerResourceModel, l *client.NetworkListener, diags *fwDiags) {
	m.ID = strValue(l.ID)
	m.Name = strValue(l.Name)
	m.Protocol = strValue(l.Protocol)
	m.TLSImplicit = boolValue(l.TLSImplicit)

	bind, d := stringSetValue(deref(l.Bind))
	diags.Append(d...)
	m.Bind = bind

	networks, d := stringSetValue(deref(l.OverrideProxyTrustedNetworks))
	diags.Append(d...)
	m.OverrideProxyTrustedNetworks = networks
}

func (r *networkListenerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkListenerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeNetworkListener, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create network listener", err.Error())
		return
	}

	var created client.NetworkListener
	if err := r.client.GetOne(ctx, client.TypeNetworkListener, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read network listener after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkListenerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkListenerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var l client.NetworkListener
	if err := r.client.GetOne(ctx, client.TypeNetworkListener, state.ID.ValueString(), &l); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read network listener", err.Error())
		return
	}
	r.fromAPI(&state, &l, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkListenerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkListenerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state networkListenerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// name is RequiresReplace; never send it on update.
	body.Name = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeNetworkListener, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update network listener", err.Error())
		return
	}

	var updated client.NetworkListener
	if err := r.client.GetOne(ctx, client.TypeNetworkListener, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read network listener after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkListenerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkListenerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeNetworkListener, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete network listener", err.Error())
	}
}

// ImportState imports a network listener by its name or opaque id.
func (r *networkListenerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveByNameOrID(ctx, r.client, client.TypeNetworkListener, req.ID,
		map[string]any{"name": req.ID})
	if err != nil {
		resp.Diagnostics.AddError("Unable to import network listener", fmt.Sprintf("%s: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
