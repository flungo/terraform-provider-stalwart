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
	_ datasource.DataSource              = &dnsRecordsDataSource{}
	_ datasource.DataSourceWithConfigure = &dnsRecordsDataSource{}
)

// NewDNSRecordsDataSource is the constructor referenced by the provider.
func NewDNSRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

type dnsRecordsDataSource struct {
	client *client.Client
}

type dnsRecordsDataSourceModel struct {
	Domain   types.String `tfsdk:"domain"`
	DomainID types.String `tfsdk:"domain_id"`
	ZoneFile types.String `tfsdk:"zone_file"`
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the DNS record recommendations for a domain. Stalwart exposes the expected " +
			"records as the read-only `dnsZoneFile` field on the Domain object; this data source returns " +
			"that zone data so it can be published to an external DNS provider.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Fully-qualified domain name to retrieve DNS records for, e.g. `example.com`.",
			},
			"domain_id": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque server-assigned identifier of the domain.",
			},
			"zone_file": schema.StringAttribute{
				Computed:    true,
				Description: "DNS zone data the server expects to be published for the domain.",
			},
		},
	}
}

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data dnsRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := d.client.QueryOne(ctx, client.TypeDomain, map[string]any{"name": data.Domain.ValueString()})
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Domain not found", "No domain named "+data.Domain.ValueString())
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

	data.DomainID = strValue(dom.ID)
	data.ZoneFile = strValue(dom.DNSZoneFile)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
