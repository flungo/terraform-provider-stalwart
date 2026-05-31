// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ datasource.DataSource              = &domainDataSource{}
	_ datasource.DataSourceWithConfigure = &domainDataSource{}
)

// NewDomainDataSource is the constructor referenced by the provider.
func NewDomainDataSource() datasource.DataSource {
	return &domainDataSource{}
}

type domainDataSource struct {
	client *client.Client
}

type domainDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Catchall       types.String `tfsdk:"catchall"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Aliases        types.List   `tfsdk:"aliases"`
	DkimManagement types.String `tfsdk:"dkim_management"`
	DNSManagement  types.String `tfsdk:"dns_management"`
	CreatedAt      types.String `tfsdk:"created_at"`
	DNSZoneFile    types.String `tfsdk:"dns_zone_file"`
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Stalwart domain by name.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Fully-qualified domain name to look up, e.g. `example.com`.",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque server-assigned identifier (ULID) of the domain.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the domain.",
			},
			"catchall": schema.StringAttribute{
				Computed:    true,
				Description: "Catch-all email address for the domain.",
			},
			"enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the domain is enabled.",
			},
			"aliases": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Alias domain names.",
			},
			"dkim_management": schema.StringAttribute{
				Computed:    true,
				Description: "DKIM management mode (`Automatic` or `Manual`).",
			},
			"dns_management": schema.StringAttribute{
				Computed:    true,
				Description: "DNS management mode (`Automatic` or `Manual`).",
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

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data domainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := d.client.QueryOne(ctx, client.TypeDomain, map[string]any{"name": data.Name.ValueString()})
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Domain not found", "No domain named "+data.Name.ValueString())
			return
		}
		resp.Diagnostics.AddError("Unable to query domain", err.Error())
		return
	}

	var dom client.Domain
	if err := d.client.GetOne(ctx, client.TypeDomain, id, &dom); err != nil {
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}

	data.ID = strValue(dom.ID)
	data.Name = strValue(dom.Name)
	data.Description = strValue(dom.Description)
	data.Catchall = strValue(dom.CatchAllAddress)
	data.Enabled = boolValue(dom.IsEnabled)
	data.CreatedAt = strValue(dom.CreatedAt)
	data.DNSZoneFile = strValue(dom.DNSZoneFile)
	if dom.DkimManagement != nil {
		data.DkimManagement = types.StringValue(dom.DkimManagement.Type)
	}
	if dom.DNSManagement != nil {
		data.DNSManagement = types.StringValue(dom.DNSManagement.Type)
	}
	aliases, di := stringListValue(deref(dom.Aliases))
	resp.Diagnostics.Append(di...)
	data.Aliases = aliases

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
