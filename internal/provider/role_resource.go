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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &roleResource{}
	_ resource.ResourceWithConfigure   = &roleResource{}
	_ resource.ResourceWithImportState = &roleResource{}
)

// NewRoleResource is the constructor referenced by the provider.
func NewRoleResource() resource.Resource {
	return &roleResource{}
}

type roleResource struct {
	client *client.Client
}

type roleResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Description         types.String `tfsdk:"description"`
	Extends             types.List   `tfsdk:"extends"`
	EnabledPermissions  types.List   `tfsdk:"enabled_permissions"`
	DisabledPermissions types.List   `tfsdk:"disabled_permissions"`
}

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart role: a named set of permissions (the `Role` JMAP object).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier (ULID) of the role.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Description of the role. This is the role's human-facing identifier.",
			},
			"extends": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Ids of roles this role extends (maps to `roleIds`).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"enabled_permissions": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Permissions explicitly enabled by this role.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"disabled_permissions": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Permissions explicitly disabled by this role; takes precedence over enabled permissions.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *roleResource) toAPI(ctx context.Context, m *roleResourceModel, diags *fwDiags) *client.Role {
	role := &client.Role{
		Description: strPtr(m.Description),
	}
	role.RoleIDs = stringSetPtr(stringSlice(ctx, m.Extends, diags))
	role.EnabledPermissions = stringSetPtr(stringSlice(ctx, m.EnabledPermissions, diags))
	role.DisabledPermissions = stringSetPtr(stringSlice(ctx, m.DisabledPermissions, diags))
	return role
}

func (r *roleResource) fromAPI(m *roleResourceModel, role *client.Role, diags *fwDiags) {
	m.ID = strValue(role.ID)
	m.Description = strValue(role.Description)

	extends, d := stringListValue(deref(role.RoleIDs))
	diags.Append(d...)
	m.Extends = extends

	enabled, d := stringListValue(deref(role.EnabledPermissions))
	diags.Append(d...)
	m.EnabledPermissions = enabled

	disabled, d := stringListValue(deref(role.DisabledPermissions))
	diags.Append(d...)
	m.DisabledPermissions = disabled
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeRole, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create role", err.Error())
		return
	}

	var created client.Role
	if err := r.client.GetOne(ctx, client.TypeRole, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read role after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var role client.Role
	if err := r.client.GetOne(ctx, client.TypeRole, state.ID.ValueString(), &role); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read role", err.Error())
		return
	}
	r.fromAPI(&state, &role, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeRole, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update role", err.Error())
		return
	}

	var updated client.Role
	if err := r.client.GetOne(ctx, client.TypeRole, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read role after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeRole, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete role", err.Error())
	}
}

// ImportState imports a role by its description or by its opaque id (ULID).
func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if client.IsULID(req.ID) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
		return
	}
	id, err := r.client.QueryOne(ctx, client.TypeRole, map[string]any{"description": req.ID})
	if err != nil {
		resp.Diagnostics.AddError("Unable to import role",
			fmt.Sprintf("%s (provide the opaque id instead if the description is ambiguous)", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
