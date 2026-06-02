// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

// Package provider implements the Terraform provider for the Stalwart mail and
// collaboration server, built on the Terraform Plugin Framework.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// Ensure StalwartProvider satisfies the provider.Provider interface.
var _ provider.Provider = &StalwartProvider{}

// Environment variable names used to source provider configuration.
const (
	envEndpoint = "STALWART_ENDPOINT"
	envToken    = "STALWART_TOKEN"
	envUsername = "STALWART_USERNAME"
	envPassword = "STALWART_PASSWORD"
)

// StalwartProvider is the provider implementation.
type StalwartProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" during acceptance testing.
	version string
}

// StalwartProviderModel maps provider schema data to a Go type.
type StalwartProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

// New returns a function that constructs the provider for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StalwartProvider{version: version}
	}
}

func (p *StalwartProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stalwart"
	resp.Version = p.version
}

func (p *StalwartProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Stalwart mail and collaboration server (v0.16+) through its JMAP management API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				Description: "Base URL of the Stalwart server, e.g. `https://mail.example.com`. " +
					"The provider appends the `/jmap` management endpoint. " +
					"May also be set with the `" + envEndpoint + "` environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Bearer token used for authentication. Takes precedence over " +
					"`username`/`password`. May also be set with the `" + envToken + "` environment variable.",
			},
			"username": schema.StringAttribute{
				Optional: true,
				Description: "Username for HTTP Basic authentication (alternative to `token`). " +
					"May also be set with the `" + envUsername + "` environment variable.",
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Password for HTTP Basic authentication (used with `username`). " +
					"May also be set with the `" + envPassword + "` environment variable.",
			},
		},
	}
}

func (p *StalwartProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg StalwartProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown values cannot be resolved during planning; bail out so the
	// practitioner gets a clear error rather than a confusing one later.
	if cfg.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Unknown Stalwart endpoint",
			"The provider cannot be configured with an unknown endpoint value.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve configuration, with explicit config taking precedence over the
	// environment.
	endpoint := firstNonEmpty(cfg.Endpoint.ValueString(), os.Getenv(envEndpoint))
	token := firstNonEmpty(cfg.Token.ValueString(), os.Getenv(envToken))
	username := firstNonEmpty(cfg.Username.ValueString(), os.Getenv(envUsername))
	password := firstNonEmpty(cfg.Password.ValueString(), os.Getenv(envPassword))

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Missing Stalwart endpoint",
			"Set the `endpoint` attribute or the "+envEndpoint+" environment variable.")
	}
	if token == "" && username == "" {
		resp.Diagnostics.AddError(
			"Missing Stalwart credentials",
			"Provide either a `token` (or "+envToken+") or a `username`/`password` pair "+
				"(or "+envUsername+"/"+envPassword+").")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(client.Config{
		Endpoint:  endpoint,
		Token:     token,
		Username:  username,
		Password:  password,
		UserAgent: "terraform-provider-stalwart/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Stalwart client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *StalwartProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewDkimSignatureResource,
		NewAccountResource,
		NewGroupResource,
		NewMailingListResource,
		NewRoleResource,
		NewDnsServerResource,
		NewAcmeProviderResource,
		NewDirectoryResource,
		NewNetworkListenerResource,
	}
}

func (p *StalwartProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainDataSource,
		NewAccountDataSource,
		NewDNSRecordsDataSource,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
