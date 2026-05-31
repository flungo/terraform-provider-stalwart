// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithConfigure   = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

// NewGroupResource is the constructor referenced by the provider.
func NewGroupResource() resource.Resource {
	return &groupResource{}
}

type groupResource struct {
	client *client.Client
}

type groupResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	DomainID      types.String `tfsdk:"domain_id"`
	Domain        types.String `tfsdk:"domain"`
	EmailAddress  types.String `tfsdk:"email_address"`
	Description   types.String `tfsdk:"description"`
	Quota         types.Int64  `tfsdk:"quota"`
	Role          types.String `tfsdk:"role"`
	RoleIDs       types.List   `tfsdk:"role_ids"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UsedDiskQuota types.Int64  `tfsdk:"used_disk_quota"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart group (the `Account` JMAP object with `@type` `Group`). " +
			"Group membership is managed from the member side, via the `member_of` attribute on " +
			"`stalwart_account`, mirroring the underlying API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier (ULID) of the group.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Group name — the local part of the email address. " +
					"Changing it replaces the group.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain_id": domainIDAttribute(),
			"domain":    domainNameAttribute(),
			"email_address": schema.StringAttribute{
				Computed:    true,
				Description: "Full email address of the group, formed as `name@domain` (server-set).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable description of the group.",
			},
			"quota": schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum disk space allocated to the group, in bytes (maps to `maxDiskQuota`).",
			},
			"role": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Default"),
				Description: "Built-in role for the group: `Default` or `Custom`. Defaults to `Default`.",
			},
			"role_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Custom role ids assigned to the group, used when `role` is `Custom`.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp of the group.",
			},
			"used_disk_quota": schema.Int64Attribute{
				Computed:    true,
				Description: "Disk space currently used by the group, in bytes (server-set).",
			},
		},
	}
}

func (r *groupResource) toAPI(ctx context.Context, m *groupResourceModel, domainID string, diags *fwDiags) *client.Account {
	typ := client.AccountTypeGroup
	grp := &client.Account{
		Type:        &typ,
		Name:        strPtr(m.Name),
		DomainID:    &domainID,
		Description: strPtr(m.Description),
		Permissions: &client.Permissions{Type: "Inherit"},
	}

	roles := &client.Roles{Type: m.Role.ValueString()}
	if roleIDs := stringSlice(ctx, m.RoleIDs, diags); roleIDs != nil {
		roles.RoleIDs = roleIDs
	}
	grp.Roles = roles

	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		grp.Quotas = map[string]int64{quotaDisk: m.Quota.ValueInt64()}
	}
	return grp
}

func (r *groupResource) fromAPI(m *groupResourceModel, grp *client.Account, diags *fwDiags) {
	m.ID = strValue(grp.ID)
	m.Name = strValue(grp.Name)
	m.DomainID = strValue(grp.DomainID)
	m.EmailAddress = strValue(grp.EmailAddress)
	m.Description = strValue(grp.Description)
	m.CreatedAt = strValue(grp.CreatedAt)
	m.UsedDiskQuota = int64Value(grp.UsedDiskQuota)

	if grp.Roles != nil {
		m.Role = types.StringValue(grp.Roles.Type)
		roleIDs, d := stringListValue(grp.Roles.RoleIDs)
		diags.Append(d...)
		m.RoleIDs = roleIDs
	}

	if q, ok := grp.Quotas[quotaDisk]; ok {
		m.Quota = types.Int64Value(q)
	} else {
		m.Quota = types.Int64Null()
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, err := resolveDomainID(ctx, r.client, plan.DomainID, plan.Domain)
	if err != nil {
		resp.Diagnostics.AddError("Unable to resolve domain", err.Error())
		return
	}

	body := r.toAPI(ctx, &plan, domainID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeAccount, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create group", err.Error())
		return
	}

	var created client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read group after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var grp client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, state.ID.ValueString(), &grp); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read group", err.Error())
		return
	}
	r.fromAPI(&state, &grp, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	body := r.toAPI(ctx, &plan, domainID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Type = nil
	body.Name = nil
	body.DomainID = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeAccount, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update group", err.Error())
		return
	}

	var updated client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read group after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeAccount, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete group", err.Error())
	}
}

// ImportState imports a group by its email address (`local@domain`) or by its
// opaque id (ULID).
func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveAccountByEmailOrID(ctx, r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import group", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
