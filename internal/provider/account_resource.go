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

// quotaDisk is the StorageQuota key used for the overall disk quota in bytes.
const quotaDisk = "maxDiskQuota"

var (
	_ resource.Resource                = &accountResource{}
	_ resource.ResourceWithConfigure   = &accountResource{}
	_ resource.ResourceWithImportState = &accountResource{}
)

// NewAccountResource is the constructor referenced by the provider.
func NewAccountResource() resource.Resource {
	return &accountResource{}
}

type accountResource struct {
	client *client.Client
}

type accountResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	DomainID      types.String `tfsdk:"domain_id"`
	Domain        types.String `tfsdk:"domain"`
	EmailAddress  types.String `tfsdk:"email_address"`
	Description   types.String `tfsdk:"description"`
	Password      types.String `tfsdk:"password"`
	Quota         types.Int64  `tfsdk:"quota"`
	Role          types.String `tfsdk:"role"`
	RoleIDs       types.List   `tfsdk:"role_ids"`
	MemberOf      types.List   `tfsdk:"member_of"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UsedDiskQuota types.Int64  `tfsdk:"used_disk_quota"`
}

func (r *accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *accountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an individual Stalwart account (the `Account` JMAP object with `@type` `User`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier (ULID) of the account.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Account name — the local part of the email address (the part before `@`). " +
					"Changing it replaces the account.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain_id": domainIDAttribute(),
			"domain":    domainNameAttribute(),
			"email_address": schema.StringAttribute{
				Computed:    true,
				Description: "Full email address of the account, formed as `name@domain` (server-set).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable description of the account.",
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Password credential for the account. Write-only: the server never " +
					"returns it, so it is not refreshed into state and out-of-band changes are not detected.",
			},
			"quota": schema.Int64Attribute{
				Optional: true,
				Description: "Maximum disk space allocated to the account, in bytes " +
					"(maps to the `maxDiskQuota` storage quota). Omit or set to 0 for unlimited.",
			},
			"role": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("User"),
				Description: "Built-in role for the account: `User`, `Admin`, or `Custom`. Defaults to `User`.",
			},
			"role_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Custom role ids assigned to the account, used when `role` is `Custom`.",
			},
			"member_of": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Ids of groups this account is a member of (maps to `memberGroupIds`).",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp of the account.",
			},
			"used_disk_quota": schema.Int64Attribute{
				Computed:    true,
				Description: "Disk space currently used by the account, in bytes (server-set).",
			},
		},
	}
}

func (r *accountResource) toAPI(ctx context.Context, m *accountResourceModel, domainID string, diags *fwDiags) *client.Account {
	typ := client.AccountTypeUser
	acct := &client.Account{
		Type:             &typ,
		Name:             strPtr(m.Name),
		DomainID:         &domainID,
		Description:      strPtr(m.Description),
		Permissions:      &client.Permissions{Type: "Inherit"},
		EncryptionAtRest: &client.TypedRef{Type: "Disabled"},
	}

	roles := &client.Roles{Type: m.Role.ValueString()}
	if roleIDs := stringSlice(ctx, m.RoleIDs, diags); roleIDs != nil {
		roles.RoleIDs = roleIDs
	}
	acct.Roles = roles

	acct.MemberGroupIDs = stringSetPtr(stringSlice(ctx, m.MemberOf, diags))

	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		acct.Quotas = map[string]int64{quotaDisk: m.Quota.ValueInt64()}
	}

	if pw := strPtr(m.Password); pw != nil {
		acct.Credentials = &client.IndexList[client.Credential]{{Type: "Password", Secret: pw}}
	}

	return acct
}

// fromAPI populates the model from the server object. The password is never
// returned and is preserved by the caller.
func (r *accountResource) fromAPI(m *accountResourceModel, acct *client.Account, diags *fwDiags) {
	m.ID = strValue(acct.ID)
	m.Name = strValue(acct.Name)
	m.DomainID = strValue(acct.DomainID)
	m.EmailAddress = strValue(acct.EmailAddress)
	m.Description = strValue(acct.Description)
	m.CreatedAt = strValue(acct.CreatedAt)
	m.UsedDiskQuota = int64Value(acct.UsedDiskQuota)

	if acct.Roles != nil {
		m.Role = types.StringValue(acct.Roles.Type)
		roleIDs, d := stringListValue(acct.Roles.RoleIDs)
		diags.Append(d...)
		m.RoleIDs = roleIDs
	}

	memberOf, d := stringListValue(deref(acct.MemberGroupIDs))
	diags.Append(d...)
	m.MemberOf = memberOf

	if q, ok := acct.Quotas[quotaDisk]; ok {
		m.Quota = types.Int64Value(q)
	} else {
		m.Quota = types.Int64Null()
	}
}

func (r *accountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountResourceModel
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
		resp.Diagnostics.AddError("Unable to create account", err.Error())
		return
	}

	var created client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read account after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var acct client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, state.ID.ValueString(), &acct); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read account", err.Error())
		return
	}
	r.fromAPI(&state, &acct, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan accountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state accountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	body := r.toAPI(ctx, &plan, domainID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// @type, name, and domainId form the immutable identity here.
	body.Type = nil
	body.Name = nil
	body.DomainID = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeAccount, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update account", err.Error())
		return
	}

	var updated client.Account
	if err := r.client.GetOne(ctx, client.TypeAccount, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read account after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeAccount, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete account", err.Error())
	}
}

// ImportState imports an account by its email address (`local@domain`) or by its
// opaque id (ULID).
func (r *accountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveAccountByEmailOrID(ctx, r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import account", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
