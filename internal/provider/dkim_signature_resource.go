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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

var (
	_ resource.Resource                = &dkimSignatureResource{}
	_ resource.ResourceWithConfigure   = &dkimSignatureResource{}
	_ resource.ResourceWithImportState = &dkimSignatureResource{}
)

// algorithmToType maps the user-facing `algorithm` value to the DkimSignature
// "@type" discriminator, and vice versa.
var (
	algorithmToType = map[string]string{
		"ed25519-sha256": "Dkim1Ed25519Sha256",
		"rsa-sha256":     "Dkim1RsaSha256",
	}
	typeToAlgorithm = map[string]string{
		"Dkim1Ed25519Sha256": "ed25519-sha256",
		"Dkim1RsaSha256":     "rsa-sha256",
	}
)

// NewDkimSignatureResource is the constructor referenced by the provider.
func NewDkimSignatureResource() resource.Resource {
	return &dkimSignatureResource{}
}

type dkimSignatureResource struct {
	client *client.Client
}

type dkimSignatureResourceModel struct {
	ID               types.String `tfsdk:"id"`
	DomainID         types.String `tfsdk:"domain_id"`
	Domain           types.String `tfsdk:"domain"`
	Selector         types.String `tfsdk:"selector"`
	Algorithm        types.String `tfsdk:"algorithm"`
	PrivateKey       types.String `tfsdk:"private_key"`
	Expiry           types.String `tfsdk:"expiry"`
	Canonicalization types.String `tfsdk:"canonicalization"`
	Headers          types.List   `tfsdk:"headers"`
	Report           types.Bool   `tfsdk:"report"`
	PublicKey        types.String `tfsdk:"public_key"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *dkimSignatureResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dkim_signature"
}

func (r *dkimSignatureResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *dkimSignatureResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DKIM signing key for a domain (the `DkimSignature` JMAP object).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Opaque server-assigned identifier (ULID) of the DKIM signature.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain_id": domainIDAttribute(),
			"domain":    domainNameAttribute(),
			"selector": schema.StringAttribute{
				Required:    true,
				Description: "Selector used to locate the DKIM public key in DNS.",
			},
			"algorithm": schema.StringAttribute{
				Required: true,
				Description: "Signing algorithm: `ed25519-sha256` or `rsa-sha256`. " +
					"Changing the algorithm replaces the signature.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"private_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "PEM-encoded private key used to sign outgoing messages.",
			},
			"expiry": schema.StringAttribute{
				Optional: true,
				Description: "Duration after which this signature expires (e.g. `90d`), " +
					"mapping to the `expire` field.",
			},
			"canonicalization": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("relaxed/relaxed"),
				Description: "Canonicalization algorithm. Defaults to `relaxed/relaxed`.",
			},
			"headers": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Message headers to include in the DKIM signature.",
			},
			"report": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to request failure reports. Defaults to `true`.",
			},
			"public_key": schema.StringAttribute{
				Computed:    true,
				Description: "PEM-encoded public key derived from the private key (server-set).",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp of the DKIM signature.",
			},
		},
	}
}

func (r *dkimSignatureResource) toAPI(ctx context.Context, m *dkimSignatureResourceModel, domainID string, diags *fwDiags) *client.DkimSignature {
	typ, ok := algorithmToType[m.Algorithm.ValueString()]
	if !ok {
		diags.AddError("Invalid DKIM algorithm",
			fmt.Sprintf("%q is not a supported algorithm; use `ed25519-sha256` or `rsa-sha256`.", m.Algorithm.ValueString()))
		return nil
	}
	sig := &client.DkimSignature{
		Type:             &typ,
		DomainID:         &domainID,
		Selector:         strPtr(m.Selector),
		PrivateKey:       &client.SecretText{Type: "Text", Secret: strPtr(m.PrivateKey)},
		Expire:           strPtr(m.Expiry),
		Canonicalization: strPtr(m.Canonicalization),
		Report:           boolPtr(m.Report),
	}
	if headers := stringSlice(ctx, m.Headers, diags); headers != nil {
		sig.Headers = stringSetPtr(headers)
	}
	return sig
}

// fromAPI populates the model from the server object. The private key is never
// returned by the server, so the configured value is preserved by the caller.
func (r *dkimSignatureResource) fromAPI(m *dkimSignatureResourceModel, sig *client.DkimSignature, diags *fwDiags) {
	m.ID = strValue(sig.ID)
	m.DomainID = strValue(sig.DomainID)
	m.Selector = strValue(sig.Selector)
	if sig.Type != nil {
		if alg, ok := typeToAlgorithm[*sig.Type]; ok {
			m.Algorithm = types.StringValue(alg)
		}
	}
	m.Expiry = strValue(sig.Expire)
	m.Canonicalization = strValue(sig.Canonicalization)
	m.Report = boolValue(sig.Report)
	m.PublicKey = strValue(sig.PublicKey)
	m.CreatedAt = strValue(sig.CreatedAt)

	headers, d := stringListValue(deref(sig.Headers))
	diags.Append(d...)
	m.Headers = headers
}

func (r *dkimSignatureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dkimSignatureResourceModel
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

	id, err := r.client.Create(ctx, client.TypeDkimSignature, body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DKIM signature", err.Error())
		return
	}

	var created client.DkimSignature
	if err := r.client.GetOne(ctx, client.TypeDkimSignature, id, &created); err != nil {
		resp.Diagnostics.AddError("Unable to read DKIM signature after create", err.Error())
		return
	}
	r.fromAPI(&plan, &created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dkimSignatureResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dkimSignatureResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var sig client.DkimSignature
	if err := r.client.GetOne(ctx, client.TypeDkimSignature, state.ID.ValueString(), &sig); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read DKIM signature", err.Error())
		return
	}
	r.fromAPI(&state, &sig, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dkimSignatureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dkimSignatureResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state dkimSignatureResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	body := r.toAPI(ctx, &plan, domainID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// @type and domainId are immutable parts of the identity here.
	body.Type = nil
	body.DomainID = nil

	id := state.ID.ValueString()
	if err := r.client.Update(ctx, client.TypeDkimSignature, id, body); err != nil {
		resp.Diagnostics.AddError("Unable to update DKIM signature", err.Error())
		return
	}

	var updated client.DkimSignature
	if err := r.client.GetOne(ctx, client.TypeDkimSignature, id, &updated); err != nil {
		resp.Diagnostics.AddError("Unable to read DKIM signature after update", err.Error())
		return
	}
	r.fromAPI(&plan, &updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dkimSignatureResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dkimSignatureResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Destroy(ctx, client.TypeDkimSignature, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete DKIM signature", err.Error())
	}
}

// ImportState imports a DKIM signature by its opaque id (ULID). DKIM signatures
// have no globally-unique human-friendly name, so only id import is supported.
func (r *dkimSignatureResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !client.IsULID(req.ID) {
		resp.Diagnostics.AddError("Invalid DKIM signature import id",
			fmt.Sprintf("%q is not a ULID. DKIM signatures must be imported by their opaque id.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
