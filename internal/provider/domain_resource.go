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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithConfigure   = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

// NewDomainResource is the constructor referenced by the provider.
func NewDomainResource() resource.Resource {
	return &domainResource{}
}

type domainResource struct {
	client *client.Client
}

type domainResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	Catchall                types.String `tfsdk:"catchall"`
	Aliases                 types.Set    `tfsdk:"aliases"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	Subaddressing           types.String `tfsdk:"subaddressing"`
	DirectoryID             types.String `tfsdk:"directory_id"`
	CertificateManagement   types.String `tfsdk:"certificate_management"`
	AcmeProviderID          types.String `tfsdk:"acme_provider_id"`
	SubjectAlternativeNames types.Set    `tfsdk:"subject_alternative_names"`
	DkimManagement          types.String `tfsdk:"dkim_management"`
	DNSManagement           types.String `tfsdk:"dns_management"`
	DNSServerID             types.String `tfsdk:"dns_server_id"`
	PublishRecords          types.Bool   `tfsdk:"publish_records"`
	DNSOrigin               types.String `tfsdk:"dns_origin"`
	AllowRelaying           types.Bool   `tfsdk:"allow_relaying"`
	ReportAddress           types.String `tfsdk:"report_address"`
	CreatedAt               types.String `tfsdk:"created_at"`
	DNSZoneFile             types.String `tfsdk:"dns_zone_file"`
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart email domain (the `Domain` JMAP object).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier of the domain.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Fully-qualified domain name, e.g. `example.com`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable description of the domain.",
			},
			"catchall": schema.StringAttribute{
				Optional: true,
				Description: "Catch-all email address that receives messages addressed to unknown " +
					"local recipients (maps to the `catchAllAddress` field).",
			},
			"aliases": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional domain names that are aliases of this domain. " +
					"Defaults to an empty list.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether this domain is enabled. Defaults to `true`.",
			},
			"subaddressing": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Enabled"),
				Description: "Sub-addressing (plus addressing) mode: `Enabled` or `Disabled`. Defaults to `Enabled`.",
			},
			"directory_id": schema.StringAttribute{
				Optional: true,
				Description: "Opaque id of the authentication directory used to resolve accounts in this domain. " +
					"When omitted the server uses the global directory.",
			},
			"certificate_management": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Manual"),
				Description: "TLS certificate management mode: `Manual` or `Automatic`. Defaults to `Manual`.",
			},
			"acme_provider_id": schema.StringAttribute{
				Optional:    true,
				Description: "ACME provider id used when `certificate_management` is `Automatic`.",
			},
			"subject_alternative_names": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional subject alternative names (SANs) to include in the TLS certificate " +
					"when `certificate_management` is `Automatic`.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"dkim_management": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Automatic"),
				Description: "DKIM key management mode: `Automatic` or `Manual`. Defaults to `Automatic`.",
			},
			"dns_management": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Manual"),
				Description: "DNS record management mode: `Manual` or `Automatic`. Defaults to `Manual`.",
			},
			"dns_server_id": schema.StringAttribute{
				Optional:    true,
				Description: "DNS server id used when `dns_management` is `Automatic`.",
			},
			"publish_records": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether to automatically publish DNS records when `dns_management` is `Automatic`.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"dns_origin": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "DNS origin (zone root) used when `dns_management` is `Automatic`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"allow_relaying": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to allow relaying for non-local recipients. Defaults to `false`.",
			},
			"report_address": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Address to receive DMARC, TLS-RPT and CAA reports for this domain " +
					"(maps to `reportAddressUri`). Defaults to `mailto:postmaster`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp of the domain.",
			},
			"dns_zone_file": schema.StringAttribute{
				Computed:    true,
				Description: "Current DNS zone data the server expects to be published for the domain.",
			},
		},
	}
}

// toAPI builds a client.Domain from the model for create/update payloads. The
// id and server-set fields are intentionally excluded.
func (r *domainResource) toAPI(ctx context.Context, m *domainResourceModel, diags *fwDiags) *client.Domain {
	d := &client.Domain{
		Name:                  strPtr(m.Name),
		Description:           strPtr(m.Description),
		CatchAllAddress:       strPtr(m.Catchall),
		IsEnabled:             boolPtr(m.Enabled),
		AllowRelaying:         boolPtr(m.AllowRelaying),
		ReportAddressURI:      strPtr(m.ReportAddress),
		DirectoryID:           strPtr(m.DirectoryID),
		SubAddressing:         &client.TypedRef{Type: m.Subaddressing.ValueString()},
		DkimManagement:        &client.TypedRef{Type: m.DkimManagement.ValueString()},
		CertificateManagement: certManagementRef(ctx, m.CertificateManagement.ValueString(), m.AcmeProviderID, m.SubjectAlternativeNames, diags),
		DNSManagement:         dnsManagementRef(m.DNSManagement.ValueString(), m.DNSServerID, m.PublishRecords, m.DNSOrigin),
	}
	d.Aliases = stringSetPtr(stringSetSlice(ctx, m.Aliases, diags))
	return d
}

// certManagementRef builds a TypedRef for the certificate management field,
// attaching the ACME provider id and any extra SANs when mode is Automatic.
func certManagementRef(ctx context.Context, kind string, acmeProviderID types.String, sans types.Set, diags *fwDiags) *client.TypedRef {
	ref := &client.TypedRef{Type: kind}
	if kind == "Automatic" {
		ref.AcmeProviderID = strPtr(acmeProviderID)
		if s := stringSetSlice(ctx, sans, diags); s != nil {
			ref.SubjectAlternativeNames = stringSetPtr(s)
		}
	}
	return ref
}

// dnsManagementRef builds a TypedRef for the DNS management field, attaching
// the DNS server id, publish flag, and origin when mode is Automatic.
func dnsManagementRef(kind string, dnsServerID types.String, publishRecords types.Bool, origin types.String) *client.TypedRef {
	ref := &client.TypedRef{Type: kind}
	if kind == "Automatic" {
		ref.DNSServerID = strPtr(dnsServerID)
		ref.PublishRecords = boolPtr(publishRecords)
		ref.Origin = strPtr(origin)
	}
	return ref
}

// fromAPI populates the model from a client.Domain returned by the server.
func (r *domainResource) fromAPI(m *domainResourceModel, d *client.Domain, diags *fwDiags) {
	m.ID = strValue(d.ID)
	m.Name = strValue(d.Name)
	m.Description = strValue(d.Description)
	m.Catchall = strValue(d.CatchAllAddress)
	m.Enabled = boolValue(d.IsEnabled)
	m.AllowRelaying = boolValue(d.AllowRelaying)
	m.ReportAddress = strValue(d.ReportAddressURI)
	m.DirectoryID = strValue(d.DirectoryID)
	m.CreatedAt = strValue(d.CreatedAt)
	m.DNSZoneFile = strValue(d.DNSZoneFile)

	if d.SubAddressing != nil {
		m.Subaddressing = types.StringValue(d.SubAddressing.Type)
	}
	if d.DkimManagement != nil {
		m.DkimManagement = types.StringValue(d.DkimManagement.Type)
	}
	if d.CertificateManagement != nil {
		m.CertificateManagement = types.StringValue(d.CertificateManagement.Type)
		m.AcmeProviderID = strValue(d.CertificateManagement.AcmeProviderID)
		sans, d2 := stringSetValue(deref(d.CertificateManagement.SubjectAlternativeNames))
		diags.Append(d2...)
		m.SubjectAlternativeNames = sans
	} else {
		emptySet, d2 := stringSetValue(nil)
		diags.Append(d2...)
		m.SubjectAlternativeNames = emptySet
	}
	if d.DNSManagement != nil {
		m.DNSManagement = types.StringValue(d.DNSManagement.Type)
		m.DNSServerID = strValue(d.DNSManagement.DNSServerID)
		m.PublishRecords = boolValue(d.DNSManagement.PublishRecords)
		m.DNSOrigin = strValue(d.DNSManagement.Origin)
	} else {
		m.PublishRecords = types.BoolNull()
		m.DNSOrigin = types.StringNull()
	}

	aliases, d2 := stringSetValue(deref(d.Aliases))
	diags.Append(d2...)
	m.Aliases = aliases
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeDomain, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create domain", err.Error())
		return
	}

	var created client.Domain
	if err := r.client.GetOne(ctx, client.TypeDomain, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read domain after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var d client.Domain
	if err := r.client.GetOne(ctx, client.TypeDomain, state.ID.ValueString(), &d); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}
	r.fromAPI(&state, &d, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// Name is the immutable identity (RequiresReplace); never send it on update.
	body.Name = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeDomain, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update domain", err.Error())
		return
	}

	var updated client.Domain
	if err := r.client.GetOne(ctx, client.TypeDomain, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read domain after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeDomain, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete domain", err.Error())
	}
}

// ImportState imports a domain by its name or by its opaque id.
func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveByNameOrID(ctx, r.client, client.TypeDomain, req.ID,
		map[string]any{"name": req.ID})
	if err != nil {
		resp.Diagnostics.AddError("Unable to import domain", fmt.Sprintf("%s: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
