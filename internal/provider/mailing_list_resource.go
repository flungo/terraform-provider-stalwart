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
	_ resource.Resource                = &mailingListResource{}
	_ resource.ResourceWithConfigure   = &mailingListResource{}
	_ resource.ResourceWithImportState = &mailingListResource{}
)

// NewMailingListResource is the constructor referenced by the provider.
func NewMailingListResource() resource.Resource {
	return &mailingListResource{}
}

type mailingListResource struct {
	client *client.Client
}

type mailingListResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	DomainID     types.String `tfsdk:"domain_id"`
	Domain       types.String `tfsdk:"domain"`
	EmailAddress types.String `tfsdk:"email_address"`
	Description  types.String `tfsdk:"description"`
	Recipients   types.List   `tfsdk:"recipients"`
}

func (r *mailingListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mailing_list"
}

func (r *mailingListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *mailingListResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart mailing list (the `MailingList` JMAP object).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier (ULID) of the mailing list.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Mailing list name — the local part of the email address. " +
					"Changing it replaces the mailing list.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain_id": domainIDAttribute(),
			"domain":    domainNameAttribute(),
			"email_address": schema.StringAttribute{
				Computed:    true,
				Description: "Full email address of the mailing list, formed as `name@domain` (server-set).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable description of the mailing list.",
			},
			"recipients": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Email addresses that are members of the mailing list.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *mailingListResource) toAPI(ctx context.Context, m *mailingListResourceModel, domainID string, diags *fwDiags) *client.MailingList {
	list := &client.MailingList{
		Name:        strPtr(m.Name),
		DomainID:    &domainID,
		Description: strPtr(m.Description),
	}
	list.Recipients = stringSetPtr(stringSlice(ctx, m.Recipients, diags))
	return list
}

func (r *mailingListResource) fromAPI(m *mailingListResourceModel, list *client.MailingList, diags *fwDiags) {
	m.ID = strValue(list.ID)
	m.Name = strValue(list.Name)
	m.DomainID = strValue(list.DomainID)
	m.EmailAddress = strValue(list.EmailAddress)
	m.Description = strValue(list.Description)

	recipients, d := stringListValue(deref(list.Recipients))
	diags.Append(d...)
	m.Recipients = recipients
}

func (r *mailingListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mailingListResourceModel
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

	id, err := r.client.Create(ctx, client.TypeMailingList, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create mailing list", err.Error())
		return
	}

	var created client.MailingList
	if err := r.client.GetOne(ctx, client.TypeMailingList, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read mailing list after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mailingListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mailingListResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list client.MailingList
	if err := r.client.GetOne(ctx, client.TypeMailingList, state.ID.ValueString(), &list); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read mailing list", err.Error())
		return
	}
	r.fromAPI(&state, &list, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mailingListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mailingListResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state mailingListResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	body := r.toAPI(ctx, &plan, domainID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Name = nil
	body.DomainID = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeMailingList, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update mailing list", err.Error())
		return
	}

	var updated client.MailingList
	if err := r.client.GetOne(ctx, client.TypeMailingList, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read mailing list after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mailingListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mailingListResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeMailingList, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete mailing list", err.Error())
	}
}

// ImportState imports a mailing list by its email address or by its opaque id
// (ULID). The MailingList/query method only exposes a free-text filter (no
// name/domainId conditions), so a non-ULID import id is matched as free text and
// must resolve to exactly one mailing list.
func (r *mailingListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if client.IsULID(req.ID) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
		return
	}
	id, err := r.client.QueryOne(ctx, client.TypeMailingList, map[string]any{"text": req.ID})
	if err != nil {
		resp.Diagnostics.AddError("Unable to import mailing list",
			fmt.Sprintf("%s (provide the opaque id instead if the address is ambiguous)", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
