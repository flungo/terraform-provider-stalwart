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
	TypeDomain          = "Domain"
	TypeAccount         = "Account"
	TypeDkimSignature   = "DkimSignature"
	TypeMailingList     = "MailingList"
	TypeRole            = "Role"
	TypeDnsServer       = "DnsServer"
	TypeAcmeProvider    = "AcmeProvider"
	TypeDirectory       = "Directory"
	TypeNetworkListener = "NetworkListener"
)

// Account @type discriminator values.
const (
	AccountTypeUser  = "User"
	AccountTypeGroup = "Group"
)

// TypedRef is a minimally-modeled tagged union value. Only the "@type"
// discriminator and a small set of companion fields used by this provider are
// represented; other companion fields are ignored on read.
//
// PublishRecords is the dnsManagement.Automatic variant's set of DNS record
// types to publish. Stalwart models it as Map<DnsRecordType>, encoded on the
// wire as the same {"<value>": true} object form as StringSet — not a bool.
type TypedRef struct {
	Type                    string     `json:"@type"`
	AcmeProviderID          *string    `json:"acmeProviderId,omitempty"`
	DNSServerID             *string    `json:"dnsServerId,omitempty"`
	PublishRecords          *StringSet `json:"publishRecords,omitempty"`
	Origin                  *string    `json:"origin,omitempty"`
	SubjectAlternativeNames *StringSet `json:"subjectAlternativeNames,omitempty"`
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
	DirectoryID           *string    `json:"directoryId,omitempty"`

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

// SecretKey models a write-only secret value (DNS provider tokens, LDAP bind
// credentials, etc.). Only the inline "Value" variant is surfaced.
type SecretKey struct {
	Type   string  `json:"@type"`
	Secret *string `json:"secret,omitempty"`
}

// DnsServer models the Stalwart DnsServer object. The Type field is the
// "@type" discriminator that selects the provider (e.g. "Cloudflare", "Tsig").
// Duration fields (Timeout, TTL, etc.) are stored as milliseconds on the wire.
// Secret is used by cloud providers; Key, Host, Port, KeyName, Protocol, and
// TsigAlgorithm are used by the Tsig variant.
type DnsServer struct {
	ID                 *string    `json:"id,omitempty"`
	Type               *string    `json:"@type,omitempty"`
	Description        *string    `json:"description,omitempty"`
	Secret             *SecretKey `json:"secret,omitempty"`
	Timeout            *int64     `json:"timeout,omitempty"`
	TTL                *int64     `json:"ttl,omitempty"`
	PollingInterval    *int64     `json:"pollingInterval,omitempty"`
	PropagationTimeout *int64     `json:"propagationTimeout,omitempty"`
	PropagationDelay   *int64     `json:"propagationDelay,omitempty"`

	// Tsig variant fields.
	Host          *string    `json:"host,omitempty"`
	Port          *int64     `json:"port,omitempty"`
	KeyName       *string    `json:"keyName,omitempty"`
	Key           *SecretKey `json:"key,omitempty"`
	Protocol      *string    `json:"protocol,omitempty"`
	TsigAlgorithm *string    `json:"tsigAlgorithm,omitempty"`
}

// AcmeProvider models the Stalwart AcmeProvider object. ChallengeType selects
// the ACME challenge method; RenewBefore is a fraction-of-lifetime enum.
// AccountKey and AccountUri are server-set after ACME registration.
type AcmeProvider struct {
	ID            *string    `json:"id,omitempty"`
	ChallengeType *string    `json:"challengeType,omitempty"`
	Contact       *StringSet `json:"contact,omitempty"`
	Directory     *string    `json:"directory,omitempty"`
	RenewBefore   *string    `json:"renewBefore,omitempty"`
	MaxRetries    *int64     `json:"maxRetries,omitempty"`

	// Server-set after ACME account registration.
	AccountKey *string `json:"accountKey,omitempty"`
	AccountUri *string `json:"accountUri,omitempty"`
}

// Directory models a Stalwart authentication directory (the "Directory" JMAP
// object). The Type field is the "@type" discriminator: "Ldap" or "Oidc".
// LDAP-specific and OIDC-specific fields coexist in the same struct; unused
// fields are omitted from the wire via omitempty.
type Directory struct {
	ID          *string `json:"id,omitempty"`
	Type        *string `json:"@type,omitempty"`
	Description *string `json:"description,omitempty"`

	// LDAP variant fields.
	URL             *string    `json:"url,omitempty"`
	BaseDN          *string    `json:"baseDn,omitempty"`
	BindDN          *string    `json:"bindDn,omitempty"`
	BindSecret      *SecretKey `json:"bindSecret,omitempty"`
	FilterLogin     *string    `json:"filterLogin,omitempty"`
	FilterMailbox   *string    `json:"filterMailbox,omitempty"`
	AttrEmail       *StringSet `json:"attrEmail,omitempty"`
	AttrMemberOf    *StringSet `json:"attrMemberOf,omitempty"`
	AttrSecret      *StringSet `json:"attrSecret,omitempty"`
	AttrDescription *StringSet `json:"attrDescription,omitempty"`

	// OIDC variant fields.
	IssuerURL     *string    `json:"issuerUrl,omitempty"`
	ClaimUsername *string    `json:"claimUsername,omitempty"`
	RequireScopes *StringSet `json:"requireScopes,omitempty"`
}

// NetworkListener models a Stalwart network listener (the "NetworkListener"
// JMAP object). Protocol uses lowercase values: smtp, lmtp, http, imap, pop3,
// manageSieve. Bind is a set of "host:port" strings.
type NetworkListener struct {
	ID                           *string    `json:"id,omitempty"`
	Name                         *string    `json:"name,omitempty"`
	Bind                         *StringSet `json:"bind,omitempty"`
	Protocol                     *string    `json:"protocol,omitempty"`
	TLSImplicit                  *bool      `json:"tlsImplicit,omitempty"`
	OverrideProxyTrustedNetworks *StringSet `json:"overrideProxyTrustedNetworks,omitempty"`
}
