// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ datasource.DataSource              = &accountDataSource{}
	_ datasource.DataSourceWithConfigure = &accountDataSource{}
)

// NewAccountDataSource is the constructor referenced by the provider.
func NewAccountDataSource() datasource.DataSource {
	return &accountDataSource{}
}

type accountDataSource struct {
	client *client.Client
}

type accountDataSourceModel struct {
	Email         types.String `tfsdk:"email"`
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	DomainID      types.String `tfsdk:"domain_id"`
	EmailAddress  types.String `tfsdk:"email_address"`
	Description   types.String `tfsdk:"description"`
	Quota         types.Int64  `tfsdk:"quota"`
	MemberOf      types.List   `tfsdk:"member_of"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UsedDiskQuota types.Int64  `tfsdk:"used_disk_quota"`
}

func (d *accountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (d *accountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *accountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Stalwart account (user or group) by its email address.",
		Attributes: map[string]schema.Attribute{
			"email": schema.StringAttribute{
				Required:    true,
				Description: "Email address of the account to look up, e.g. `alice@example.com`.",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque server-assigned identifier (ULID) of the account.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Account name (the local part of the email address).",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Account type: `User` or `Group`.",
			},
			"domain_id": schema.StringAttribute{
				Computed:    true,
				Description: "Id of the domain the account belongs to.",
			},
			"email_address": schema.StringAttribute{
				Computed:    true,
				Description: "Full email address of the account.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the account.",
			},
			"quota": schema.Int64Attribute{
				Computed:    true,
				Description: "Maximum disk space allocated, in bytes (the `maxDiskQuota` storage quota).",
			},
			"member_of": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Ids of groups this account is a member of.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp of the account.",
			},
			"used_disk_quota": schema.Int64Attribute{
				Computed:    true,
				Description: "Disk space currently used, in bytes.",
			},
		},
	}
}

func (d *accountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data accountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	local, domain, ok := splitEmail(data.Email.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid account email",
			fmt.Sprintf("%q is not a valid email address (expected local@domain)", data.Email.ValueString()))
		return
	}

	domainID, err := d.client.QueryOne(ctx, client.TypeDomain, map[string]any{"name": domain})
	if err != nil {
		resp.Diagnostics.AddError("Unable to resolve domain", err.Error())
		return
	}

	id, err := d.client.QueryOne(ctx, client.TypeAccount,
		map[string]any{"name": local, "domainId": domainID})
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Account not found", "No account "+data.Email.ValueString())
			return
		}
		resp.Diagnostics.AddError("Unable to query account", err.Error())
		return
	}

	var acct client.Account
	if err := d.client.GetOne(ctx, client.TypeAccount, id, &acct); err != nil {
		resp.Diagnostics.AddError("Unable to read account", err.Error())
		return
	}

	data.ID = strValue(acct.ID)
	data.Name = strValue(acct.Name)
	data.Type = strValue(acct.Type)
	data.DomainID = strValue(acct.DomainID)
	data.EmailAddress = strValue(acct.EmailAddress)
	data.Description = strValue(acct.Description)
	data.CreatedAt = strValue(acct.CreatedAt)
	data.UsedDiskQuota = int64Value(acct.UsedDiskQuota)

	memberOf, di := stringListValue(deref(acct.MemberGroupIDs))
	resp.Diagnostics.Append(di...)
	data.MemberOf = memberOf

	if q, ok := acct.Quotas[quotaDisk]; ok {
		data.Quota = types.Int64Value(q)
	} else {
		data.Quota = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
