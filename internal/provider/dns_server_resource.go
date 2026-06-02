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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &dnsServerResource{}
	_ resource.ResourceWithConfigure   = &dnsServerResource{}
	_ resource.ResourceWithImportState = &dnsServerResource{}
)

// NewDnsServerResource is the constructor referenced by the provider.
func NewDnsServerResource() resource.Resource {
	return &dnsServerResource{}
}

type dnsServerResource struct {
	client *client.Client
}

type dnsServerResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Type               types.String `tfsdk:"type"`
	Description        types.String `tfsdk:"description"`
	Secret             types.String `tfsdk:"secret"`
	Timeout            types.String `tfsdk:"timeout"`
	TTL                types.String `tfsdk:"ttl"`
	PollingInterval    types.String `tfsdk:"polling_interval"`
	PropagationTimeout types.String `tfsdk:"propagation_timeout"`
	PropagationDelay   types.String `tfsdk:"propagation_delay"`

	// Tsig variant fields.
	Host          types.String `tfsdk:"host"`
	Port          types.Int64  `tfsdk:"port"`
	KeyName       types.String `tfsdk:"key_name"`
	Key           types.String `tfsdk:"key"`
	Protocol      types.String `tfsdk:"protocol"`
	TsigAlgorithm types.String `tfsdk:"tsig_algorithm"`
}

func (r *dnsServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_server"
}

func (r *dnsServerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *dnsServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stalwart DNS server used for automatic DNS record management (the `DnsServer` JMAP object). " +
			"Supports cloud providers (Cloudflare, DigitalOcean, etc.) via the `secret` field, " +
			"and RFC 2136 TSIG via the `host`, `port`, `key_name`, `key`, `protocol`, and `tsig_algorithm` fields.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier of the DNS server.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "DNS provider type, e.g. `Cloudflare`, `Tsig`, `DigitalOcean`. " +
					"Changing the type replaces the DNS server.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable description of the DNS server. Required and must be non-empty.",
			},
			"secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "API token or key used to authenticate with the DNS provider. " +
					"Used by cloud providers (Cloudflare, DigitalOcean, etc.). " +
					"Write-only: the server never returns it, so out-of-band changes are not detected.",
			},
			"timeout": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Request timeout for DNS API calls (e.g. `30s`, `1m`). Uses Stalwart duration syntax.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ttl": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TTL to set on managed DNS records (e.g. `300s`, `5m`). Uses Stalwart duration syntax.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"polling_interval": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Interval between DNS propagation checks (e.g. `10s`). Uses Stalwart duration syntax.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"propagation_timeout": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum time to wait for DNS propagation (e.g. `2m`). Uses Stalwart duration syntax.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"propagation_delay": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Initial delay before checking DNS propagation (e.g. `5s`). Uses Stalwart duration syntax.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// Tsig variant attributes.
			"host": schema.StringAttribute{
				Optional:    true,
				Description: "IP address of the authoritative DNS server. Required for `Tsig` type.",
			},
			"port": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Port of the authoritative DNS server (default 53). Used by `Tsig` type.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"key_name": schema.StringAttribute{
				Optional:    true,
				Description: "TSIG key name, e.g. `tsig-key.`. Required for `Tsig` type.",
			},
			"key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Base64-encoded TSIG shared secret. Required for `Tsig` type. " +
					"Write-only: the server never returns it, so out-of-band changes are not detected.",
			},
			"protocol": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Transport protocol for TSIG DNS updates: `udp` or `tcp` (default `udp`).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tsig_algorithm": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "TSIG HMAC algorithm, e.g. `hmac-sha256`, `hmac-sha512` (default `hmac-md5`). " +
					"Used by `Tsig` type.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *dnsServerResource) toAPI(m *dnsServerResourceModel, diags *fwDiags) *client.DnsServer {
	typ := m.Type.ValueString()
	desc := m.Description.ValueString()
	srv := &client.DnsServer{
		Type:        &typ,
		Description: &desc,
	}

	switch typ {
	case "Tsig":
		srv.Host = strPtr(m.Host)
		if !m.Port.IsNull() && !m.Port.IsUnknown() {
			v := m.Port.ValueInt64()
			srv.Port = &v
		}
		srv.KeyName = strPtr(m.KeyName)
		if s := strPtr(m.Key); s != nil {
			srv.Key = &client.SecretKey{Type: "Value", Secret: s}
		}
		srv.Protocol = strPtr(m.Protocol)
		srv.TsigAlgorithm = strPtr(m.TsigAlgorithm)
	default:
		if s := strPtr(m.Secret); s != nil {
			srv.Secret = &client.SecretKey{Type: "Value", Secret: s}
		}
	}

	if s := strPtr(m.Timeout); s != nil {
		ms, err := parseDuration(*s)
		if err != nil {
			diags.AddError("Invalid timeout", err.Error())
			return nil
		}
		srv.Timeout = &ms
	}
	if s := strPtr(m.TTL); s != nil {
		ms, err := parseDuration(*s)
		if err != nil {
			diags.AddError("Invalid ttl", err.Error())
			return nil
		}
		srv.TTL = &ms
	}
	if s := strPtr(m.PollingInterval); s != nil {
		ms, err := parseDuration(*s)
		if err != nil {
			diags.AddError("Invalid polling_interval", err.Error())
			return nil
		}
		srv.PollingInterval = &ms
	}
	if s := strPtr(m.PropagationTimeout); s != nil {
		ms, err := parseDuration(*s)
		if err != nil {
			diags.AddError("Invalid propagation_timeout", err.Error())
			return nil
		}
		srv.PropagationTimeout = &ms
	}
	if s := strPtr(m.PropagationDelay); s != nil {
		ms, err := parseDuration(*s)
		if err != nil {
			diags.AddError("Invalid propagation_delay", err.Error())
			return nil
		}
		srv.PropagationDelay = &ms
	}
	return srv
}

// fromAPI populates the model from the server object. Secret and Key are never
// returned by the server and are preserved from the existing model state.
// Type-specific fields not applicable to the active variant are set to null.
func (r *dnsServerResource) fromAPI(m *dnsServerResourceModel, srv *client.DnsServer) {
	m.ID = strValue(srv.ID)
	m.Type = strValue(srv.Type)
	m.Description = strValue(srv.Description)

	if srv.Timeout != nil {
		m.Timeout = types.StringValue(formatDuration(*srv.Timeout))
	} else {
		m.Timeout = types.StringNull()
	}
	if srv.TTL != nil {
		m.TTL = types.StringValue(formatDuration(*srv.TTL))
	} else {
		m.TTL = types.StringNull()
	}
	if srv.PollingInterval != nil {
		m.PollingInterval = types.StringValue(formatDuration(*srv.PollingInterval))
	} else {
		m.PollingInterval = types.StringNull()
	}
	if srv.PropagationTimeout != nil {
		m.PropagationTimeout = types.StringValue(formatDuration(*srv.PropagationTimeout))
	} else {
		m.PropagationTimeout = types.StringNull()
	}
	if srv.PropagationDelay != nil {
		m.PropagationDelay = types.StringValue(formatDuration(*srv.PropagationDelay))
	} else {
		m.PropagationDelay = types.StringNull()
	}

	switch deref(srv.Type) {
	case "Tsig":
		// Secret does not apply to TSIG.
		m.Secret = types.StringNull()
		m.Host = strValue(srv.Host)
		if srv.Port != nil {
			m.Port = types.Int64Value(*srv.Port)
		} else {
			m.Port = types.Int64Null()
		}
		m.KeyName = strValue(srv.KeyName)
		// Key is write-only; preserve from existing state.
		m.Protocol = strValue(srv.Protocol)
		m.TsigAlgorithm = strValue(srv.TsigAlgorithm)
	default:
		// Key and TSIG fields do not apply to cloud providers.
		m.Key = types.StringNull()
		m.Host = types.StringNull()
		m.Port = types.Int64Null()
		m.KeyName = types.StringNull()
		m.Protocol = types.StringNull()
		m.TsigAlgorithm = types.StringNull()
		// Secret is write-only; preserve from existing state.
	}
}

func (r *dnsServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(&plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Create(ctx, client.TypeDnsServer, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS server", err.Error())
		return
	}

	var created client.DnsServer
	if err := r.client.GetOne(ctx, client.TypeDnsServer, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read DNS server after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var srv client.DnsServer
	if err := r.client.GetOne(ctx, client.TypeDnsServer, state.ID.ValueString(), &srv); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read DNS server", err.Error())
		return
	}
	r.fromAPI(&state, &srv)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state dnsServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := r.toAPI(&plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// @type is immutable (RequiresReplace); never send it on update.
	body.Type = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeDnsServer, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update DNS server", err.Error())
		return
	}

	var updated client.DnsServer
	if err := r.client.GetOne(ctx, client.TypeDnsServer, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read DNS server after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeDnsServer, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete DNS server", err.Error())
	}
}

// ImportState imports a DNS server by its opaque id.
func (r *dnsServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !client.IsID(req.ID) {
		resp.Diagnostics.AddError("Invalid DNS server import id",
			fmt.Sprintf("%q is not a valid object id. DNS servers must be imported by their opaque id.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
