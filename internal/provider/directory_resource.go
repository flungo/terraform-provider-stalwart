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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &directoryResource{}
	_ resource.ResourceWithConfigure   = &directoryResource{}
	_ resource.ResourceWithImportState = &directoryResource{}
)

// NewDirectoryResource is the constructor referenced by the provider.
func NewDirectoryResource() resource.Resource {
	return &directoryResource{}
}

type directoryResource struct {
	client *client.Client
}

type directoryResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`

	// LDAP variant.
	URL             types.String `tfsdk:"url"`
	BaseDN          types.String `tfsdk:"base_dn"`
	BindDN          types.String `tfsdk:"bind_dn"`
	BindSecret      types.String `tfsdk:"bind_secret"`
	FilterLogin     types.String `tfsdk:"filter_login"`
	FilterMailbox   types.String `tfsdk:"filter_mailbox"`
	AttrEmail       types.Set    `tfsdk:"attr_email"`
	AttrMemberOf    types.Set    `tfsdk:"attr_member_of"`
	AttrSecret      types.Set    `tfsdk:"attr_secret"`
	AttrDescription types.Set    `tfsdk:"attr_description"`

	// OIDC variant.
	IssuerURL     types.String `tfsdk:"issuer_url"`
	ClaimUsername types.String `tfsdk:"claim_username"`
	RequireScopes types.Set    `tfsdk:"require_scopes"`
}

func (r *directoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_directory"
}

func (r *directoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *directoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart authentication directory backend (the `Directory` JMAP object). " +
			"Supports LDAP (`Ldap`) and OIDC (`Oidc`) directory types.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier of the directory.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "Directory backend type: `Ldap` or `Oidc`. " +
					"Changing the type replaces the directory.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable description of the directory. Required and must be non-empty.",
			},

			// LDAP attributes.
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "LDAP server URL, e.g. `ldap://auth.example.com:389`. Required for `Ldap` type.",
			},
			"base_dn": schema.StringAttribute{
				Optional:    true,
				Description: "Base distinguished name for LDAP searches, e.g. `dc=example,dc=com`. Required for `Ldap` type.",
			},
			"bind_dn": schema.StringAttribute{
				Optional:    true,
				Description: "Distinguished name used to bind to the LDAP server for lookups.",
			},
			"bind_secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Password for the LDAP bind DN. Write-only: the server never returns it, " +
					"so out-of-band changes are not detected.",
			},
			"filter_login": schema.StringAttribute{
				Optional:    true,
				Description: "LDAP filter template for login lookups, e.g. `(&(objectClass=inetOrgPerson)(uid=?))`.",
			},
			"filter_mailbox": schema.StringAttribute{
				Optional:    true,
				Description: "LDAP filter template for mailbox lookups, e.g. `(&(objectClass=inetOrgPerson)(mail=?))`.",
			},
			"attr_email": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "LDAP attribute(s) containing email addresses.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"attr_member_of": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "LDAP attribute(s) listing group memberships (`attrMemberOf` on the wire).",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"attr_secret": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "LDAP attribute(s) containing the account password hash.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"attr_description": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "LDAP attribute(s) containing a human-readable account description.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},

			// OIDC attributes.
			"issuer_url": schema.StringAttribute{
				Optional:    true,
				Description: "OIDC issuer URL, e.g. `https://auth.example.com`. Required for `Oidc` type.",
			},
			"claim_username": schema.StringAttribute{
				Optional:    true,
				Description: "OIDC claim that contains the username, e.g. `preferred_username`.",
			},
			"require_scopes": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "OIDC scopes required for authentication.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *directoryResource) toAPI(ctx context.Context, m *directoryResourceModel, diags *fwDiags) *client.Directory {
	typ := m.Type.ValueString()
	d := &client.Directory{
		Type:        &typ,
		Description: strPtr(m.Description),
	}
	switch typ {
	case "Ldap":
		d.URL = strPtr(m.URL)
		d.BaseDN = strPtr(m.BaseDN)
		d.BindDN = strPtr(m.BindDN)
		if s := strPtr(m.BindSecret); s != nil {
			d.BindSecret = &client.SecretKey{Type: "Value", Secret: s}
		}
		d.FilterLogin = strPtr(m.FilterLogin)
		d.FilterMailbox = strPtr(m.FilterMailbox)
		if s := stringSetSlice(ctx, m.AttrEmail, diags); s != nil {
			d.AttrEmail = stringSetPtr(s)
		}
		if s := stringSetSlice(ctx, m.AttrMemberOf, diags); s != nil {
			d.AttrMemberOf = stringSetPtr(s)
		}
		if s := stringSetSlice(ctx, m.AttrSecret, diags); s != nil {
			d.AttrSecret = stringSetPtr(s)
		}
		if s := stringSetSlice(ctx, m.AttrDescription, diags); s != nil {
			d.AttrDescription = stringSetPtr(s)
		}
	case "Oidc":
		d.IssuerURL = strPtr(m.IssuerURL)
		d.ClaimUsername = strPtr(m.ClaimUsername)
		if s := stringSetSlice(ctx, m.RequireScopes, diags); s != nil {
			d.RequireScopes = stringSetPtr(s)
		}
	}
	return d
}

// fromAPI populates the model from the server object. The bind secret is never
// returned by the server and is preserved by the caller. All type-specific
// fields that do not apply to the active variant are explicitly zeroed so that
// Optional+Computed set attributes don't remain as the unknown plan value.
func (r *directoryResource) fromAPI(m *directoryResourceModel, d *client.Directory, diags *fwDiags) {
	m.ID = strValue(d.ID)
	m.Type = strValue(d.Type)
	m.Description = strValue(d.Description)

	// Zero all type-specific fields first; the active variant block below fills
	// in the ones that are relevant. This prevents Optional+Computed set fields
	// from remaining as unknown in state when the server omits them.
	emptySet, d2 := stringSetValue(nil)
	diags.Append(d2...)
	m.URL = types.StringNull()
	m.BaseDN = types.StringNull()
	m.BindDN = types.StringNull()
	m.FilterLogin = types.StringNull()
	m.FilterMailbox = types.StringNull()
	m.AttrEmail = emptySet
	m.AttrMemberOf = emptySet
	m.AttrSecret = emptySet
	m.AttrDescription = emptySet
	m.IssuerURL = types.StringNull()
	m.ClaimUsername = types.StringNull()
	m.RequireScopes = emptySet

	switch deref(d.Type) {
	case "Ldap":
		m.URL = strValue(d.URL)
		m.BaseDN = strValue(d.BaseDN)
		m.BindDN = strValue(d.BindDN)
		m.FilterLogin = strValue(d.FilterLogin)
		m.FilterMailbox = strValue(d.FilterMailbox)
		attrEmail, d2 := stringSetValue(deref(d.AttrEmail))
		diags.Append(d2...)
		m.AttrEmail = attrEmail
		attrMemberOf, d2 := stringSetValue(deref(d.AttrMemberOf))
		diags.Append(d2...)
		m.AttrMemberOf = attrMemberOf
		attrSecret, d2 := stringSetValue(deref(d.AttrSecret))
		diags.Append(d2...)
		m.AttrSecret = attrSecret
		attrDesc, d2 := stringSetValue(deref(d.AttrDescription))
		diags.Append(d2...)
		m.AttrDescription = attrDesc
	case "Oidc":
		m.IssuerURL = strValue(d.IssuerURL)
		m.ClaimUsername = strValue(d.ClaimUsername)
		scopes, d2 := stringSetValue(deref(d.RequireScopes))
		diags.Append(d2...)
		m.RequireScopes = scopes
	}
}

func (r *directoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan directoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeDirectory, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create directory", err.Error())
		return
	}

	var created client.Directory
	if err := r.client.GetOne(ctx, client.TypeDirectory, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read directory after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *directoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state directoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var d client.Directory
	if err := r.client.GetOne(ctx, client.TypeDirectory, state.ID.ValueString(), &d); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read directory", err.Error())
		return
	}
	r.fromAPI(&state, &d, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *directoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan directoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state directoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// @type is RequiresReplace; never send it on update.
	body.Type = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeDirectory, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update directory", err.Error())
		return
	}

	var updated client.Directory
	if err := r.client.GetOne(ctx, client.TypeDirectory, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read directory after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *directoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state directoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeDirectory, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete directory", err.Error())
	}
}

// ImportState imports a directory by its opaque id.
func (r *directoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !client.IsID(req.ID) {
		resp.Diagnostics.AddError("Invalid directory import id",
			fmt.Sprintf("%q is not a valid object id. Directories must be imported by their opaque id.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
