// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

// This file models the subset of Stalwart management objects exposed by this
// provider. Field names and JSON tags follow the Stalwart schema reference at
// https://stalw.art/docs/ref/object/. Optional fields use pointers with
// "omitempty" so that partial updates only transmit the properties that are
// being changed, and so that server-set fields are omitted from create/update
// payloads.
//
// NOTE: Several objects carry additional, less common fields (e.g. enterprise
// tenant association, encryption settings, custom sub-addressing expressions)
// that are not yet surfaced by the provider. These are intentionally omitted
// for the initial resource set and are marked with TODO where relevant.

// Object type names. The wire method names are derived from these with the
// "x:" prefix (see MethodPrefix), e.g. "x:Domain/get".
const (
	TypeDomain        = "Domain"
	TypeAccount       = "Account"
	TypeDkimSignature = "DkimSignature"
	TypeMailingList   = "MailingList"
	TypeRole          = "Role"
)

// Account @type discriminator values.
const (
	AccountTypeUser  = "User"
	AccountTypeGroup = "Group"
)

// TypedRef is a minimally-modeled tagged union value. Only the "@type"
// discriminator and a small set of companion fields used by this provider are
// represented; other companion fields are ignored on read.
type TypedRef struct {
	Type           string  `json:"@type"`
	AcmeProviderID *string `json:"acmeProviderId,omitempty"`
	DNSServerID    *string `json:"dnsServerId,omitempty"`
}

// Domain models the Stalwart Domain object.
type Domain struct {
	ID                    *string    `json:"id,omitempty"`
	Name                  *string    `json:"name,omitempty"`
	Description           *string    `json:"description,omitempty"`
	Aliases               *StringSet `json:"aliases,omitempty"`
	IsEnabled             *bool      `json:"isEnabled,omitempty"`
	CatchAllAddress       *string    `json:"catchAllAddress,omitempty"`
	AllowRelaying         *bool      `json:"allowRelaying,omitempty"`
	ReportAddressURI      *string    `json:"reportAddressUri,omitempty"`
	SubAddressing         *TypedRef  `json:"subAddressing,omitempty"`
	CertificateManagement *TypedRef  `json:"certificateManagement,omitempty"`
	DkimManagement        *TypedRef  `json:"dkimManagement,omitempty"`
	DNSManagement         *TypedRef  `json:"dnsManagement,omitempty"`

	// Server-set, read-only.
	CreatedAt   *string `json:"createdAt,omitempty"`
	DNSZoneFile *string `json:"dnsZoneFile,omitempty"`
}

// Roles models the UserRoles/Roles tagged union used by accounts and groups.
type Roles struct {
	Type    string    `json:"@type"`
	RoleIDs StringSet `json:"roleIds,omitempty"`
}

// Permissions models the permission-assignment tagged union for accounts and
// groups.
type Permissions struct {
	Type                string    `json:"@type"`
	EnabledPermissions  StringSet `json:"enabledPermissions,omitempty"`
	DisabledPermissions StringSet `json:"disabledPermissions,omitempty"`
}

// Credential models an account authentication credential. Only the Password
// variant is currently surfaced by the provider.
type Credential struct {
	Type   string  `json:"@type"`
	Secret *string `json:"secret,omitempty"`
}

// Account models the Stalwart Account object (both the "User" and "Group"
// variants, discriminated by Type).
type Account struct {
	ID               *string                `json:"id,omitempty"`
	Type             *string                `json:"@type,omitempty"`
	Name             *string                `json:"name,omitempty"`
	DomainID         *string                `json:"domainId,omitempty"`
	EmailAddress     *string                `json:"emailAddress,omitempty"`
	Description      *string                `json:"description,omitempty"`
	MemberGroupIDs   *StringSet             `json:"memberGroupIds,omitempty"`
	Roles            *Roles                 `json:"roles,omitempty"`
	Permissions      *Permissions           `json:"permissions,omitempty"`
	Quotas           map[string]int64       `json:"quotas,omitempty"`
	Credentials      *IndexList[Credential] `json:"credentials,omitempty"`
	EncryptionAtRest *TypedRef              `json:"encryptionAtRest,omitempty"`

	// Server-set, read-only.
	CreatedAt     *string `json:"createdAt,omitempty"`
	UsedDiskQuota *int64  `json:"usedDiskQuota,omitempty"`
}

// SecretText models the SecretText tagged union used for DKIM private keys.
// Only the inline "Text" variant is surfaced by the provider.
type SecretText struct {
	Type   string  `json:"@type"`
	Secret *string `json:"secret,omitempty"`
}

// DkimSignature models the Stalwart DkimSignature object. The Type field is the
// "@type" discriminator selecting the signing algorithm.
type DkimSignature struct {
	ID               *string     `json:"id,omitempty"`
	Type             *string     `json:"@type,omitempty"`
	DomainID         *string     `json:"domainId,omitempty"`
	Selector         *string     `json:"selector,omitempty"`
	PrivateKey       *SecretText `json:"privateKey,omitempty"`
	Expire           *int64      `json:"expire,omitempty"` // milliseconds (Stalwart Duration)
	Canonicalization *string     `json:"canonicalization,omitempty"`
	Headers          *StringSet  `json:"headers,omitempty"`
	Report           *bool       `json:"report,omitempty"`

	// Server-set, read-only.
	PublicKey *string `json:"publicKey,omitempty"`
	CreatedAt *string `json:"createdAt,omitempty"`
}

// EmailAlias models an email alias attached to an account or mailing list.
// Not yet surfaced as a managed attribute; reserved for future use.
type EmailAlias struct {
	Name        string  `json:"name"`
	DomainID    string  `json:"domainId"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Description *string `json:"description,omitempty"`
}

// MailingList models the Stalwart MailingList object.
type MailingList struct {
	ID           *string    `json:"id,omitempty"`
	Name         *string    `json:"name,omitempty"`
	DomainID     *string    `json:"domainId,omitempty"`
	EmailAddress *string    `json:"emailAddress,omitempty"`
	Description  *string    `json:"description,omitempty"`
	Recipients   *StringSet `json:"recipients,omitempty"`
}

// Role models the Stalwart Role object.
type Role struct {
	ID                  *string    `json:"id,omitempty"`
	Description         *string    `json:"description,omitempty"`
	RoleIDs             *StringSet `json:"roleIds,omitempty"`
	EnabledPermissions  *StringSet `json:"enabledPermissions,omitempty"`
	DisabledPermissions *StringSet `json:"disabledPermissions,omitempty"`
}
