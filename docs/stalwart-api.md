# Stalwart API Reference

Corrected facts about the Stalwart v0.16 and v1 JMAP management API.
Every claim here has been verified against the upstream source code and is noted with the relevant file path.

**Upstream sources** (fact-checked at these commits):

- Code: `stalwartlabs/stalwart` @ `68946d4` (2026-05-29)
- Docs: `stalwartlabs/website` @ `8044db7` (2026-05-29)

Clone these when extending the provider — do not rely on the generated `ref/object/*.md` curl snippets (they show a wrong endpoint):

```sh
git clone --depth 1 https://github.com/stalwartlabs/stalwart.git /tmp/stalwart-src
git clone --depth 1 https://github.com/stalwartlabs/website.git    /tmp/stalwart-website
# Object schemas:
ls /tmp/stalwart-website/src/content/docs/docs/ref/object/
```

The official docs site (`stalw.art/docs`) blocks automated fetches (HTTP 403).
The GitHub raw API rate-limits unauthenticated requests — prefer a shallow `git clone`.

## JMAP endpoint

**The endpoint is `/jmap`, not `/api`.**

- `crates/http/src/request.rs` routes `"jmap" =>` (POST, empty next segment) to `handle_jmap_request` (~line 83).
- `"api" =>` routes to a separate `handle_api_request` (~line 403) — a small HTTP API for auth, schema, and telemetry.
  It is NOT the management API.
- Empirically confirmed: `POST /api` → **404**, `POST /jmap` → **200** on a live v0.16 container.
- The generated `ref/object/*.md` curl snippets show `POST .../api` and are **wrong**.
  The hand-written `http/index.md` and `development/api.md` are correct.

## Capability URN

`urn:stalwart:jmap` (plus `urn:ietf:params:jmap:core`).
Not `urn:stalwart:core`.

VERIFIED: `crates/jmap-proto/src/request/capability.rs` (`Capability::Stalwart => "urn:stalwart:jmap"`).

## Server versions (v0.16 and v1)

v1.0 is a data migration from v0.16, not an in-place upgrade, and renamed or removed some management properties.
The provider supports both; the design is in [ADR 004](decisions/004-dual-server-version-support.md).

**Upstream sources** (fact-checked 2026-11-07):

- `stalwartlabs/stalwart` at tags `v0.16.27` (`aa47d8e`) and `v1.0.3` (`8b32686`, the newest 1.0.x then).
- `stalwartlabs/stalwart-migrate` @ `daf90f0`: `SCHEMA_CHANGES.md` and the schema snapshots `mapping/schema/v0.16.27.json.gz`, `v1.json.gz`.
  The snapshots are byte-identical to `resources/schema/schema.json.gz` at the two server tags.
- `stalwartlabs/website` @ `ac0c5db`: `docs/configuration/recovery-mode.md`.

**Telling them apart.**
No version string is exposed over HTTP: the JMAP session's `implementation` is the hard-coded `"Stalwart v1.0.0"` in both (`crates/jmap-proto/src/request/capability.rs`), as is `VERSION_PUBLIC` (`crates/common/src/lib.rs`).
`GET /api/schema` (any authenticated account) redirects to `/api/schema/<sha256>`, which serves the embedded schema gzip-compressed (`crates/http/src/api/mod.rs`, present since v0.16.0).
Its `fields["x:Domain"].properties` has `isEnabled` on v0.16 and `enable` on v1.

**Renamed properties the provider uses** (v0.16 → v1): `Domain.isEnabled` → `enable`; `DnsServer*.pollingInterval` → `pollInterval`; `NetworkListener.tlsImplicit` → `implicitTls`; `LdapDirectory.filterMailbox` → `filterRecipient`.

**Removed in v1**: `AcmeProvider.maxRetries`; `OidcDirectory.requireScopes`.
`AcmeProvider.description` is server-set on v0.16 (the directory URL plus the last segment of the account URI, `crates/jmap/src/registry/get.rs`) and an ordinary stored string on v1.
On v1 it is **required**: `AcmeProvider::validate` (`crates/registry/src/schema/structs_impl.rs`) rejects an empty one with `validationFailed`, `{"type":"Required","property":"description"}`, even though the schema snapshot shows it only as `mutable` with no default.
The schema document does not carry required-ness, so check `validate` in `structs_impl.rs` for any newly sent property.

**Added in v1** on `OidcDirectory`: `acceptedAudiences` (set), `unscopedTokens` (`reject` default, `interoperable`, `full`), `opaqueTokenValidation` (`none` default, `userInfo`, `introspection`), `skipAudienceValidation`, `allowIdTokenClaims`, `lenientIssuerMatch`, `allowInvalidCerts` (booleans, default `false`), plus the introspection client settings.

**Unchanged** for everything else the provider sends: query filters, the `@type` variants it uses, and the enums `AcmeChallengeType`, `AcmeRenewBefore`, `NetworkListenerProtocol`, `DkimCanonicalization`, `DnsRecordType`, `TsigAlgorithm`, `StorageQuota`.
v1 renumbered ids and enum positions, which the JMAP API never exposes: values travel by name.

**Permissions** changed: v1 removed 37 (including `jmapPrincipalCreate/Update/Destroy`, `oAuthClientRegistration`, `oAuthClientOverride`, `sysSpamLlmGet/Update`) and added 38 (including `sysOAuthGrantGet/Query/Destroy`, `sysSpamAiGet/Update`, `sysMetadataGet/Update`), per the `Permission` enum of the two snapshots.

**DNS provider credentials.**
v1 renamed several per-provider credential keys (`authToken` → `token`, `apiKey` → `apiKeyId`, ...).
The `secret` property the provider sends is the credential on the same variants in both versions (Cloudflare, DigitalOcean, DeSEC, Hetzner and 27 others) except two: Dnsimple's `secret` was a retired field on v0.16 and is gone on v1 (its credential is `authToken`/`token`), and Porkbun's live `secret` on v1 replaced `secretApiKey`.

**Recovery mode** behaves the same in both: `STALWART_RECOVERY_MODE`, `STALWART_RECOVERY_ADMIN` and `STALWART_RECOVERY_MODE_PORT` (default 8080) are read by `crates/store/src/registry/local.rs` and `crates/common/src/config/server/listener.rs` at both tags, and the v1 docs confirm the full management API is served on that listener.
The images keep `/usr/local/bin/stalwart` as the entrypoint.

## Wire method names

Method names on the wire carry an **`x:` prefix**: `x:Domain/get`, `x:Account/set`, `x:DkimSignature/query`, etc.

VERIFIED: `crates/jmap-proto/src/request/method.rs` (`format!("x:{}/{}", obj, method)` and `s.strip_prefix("x:")`).
The CLI omits the prefix in display output but it is required on the wire.

Standard JMAP semantics: `Foo/get`, `Foo/set` (create/update/destroy), `Foo/query`.
Singletons use the literal id `"singleton"`.

## Object ids

Objects are keyed by an **opaque server-generated id** (`id` field).
Child objects reference their parent via a field like `domainId` — the parent's id, not its name.

**The id is NOT a ULID.**
It is a `u64` rendered in a custom base32 alphabet:

```text
abcdefghijklmnopqrstuvwxyz792013
```

1–13 lowercase characters.
Id `0` → `"a"`.

VERIFIED: `crates/types/src/id.rs` + `crates/utils/src/codec/base32_custom.rs`.

`client.IsID(s)` in this provider recognises this alphabet to distinguish an opaque id from a human-friendly reference (name, email, description).
Human refs always contain `.`, `@`, space, or an uppercase letter — none of which are in the alphabet.

## Accounts and groups

**Accounts and groups are the same `Account` object**, discriminated by `@type`:

- `"User"` → account
- `"Group"` → group

There is no separate Group JMAP object type.

## DNS records

DNS recommendations are **not a separate JMAP method**.
They are the read-only `dnsZoneFile` text field on the `Domain` object.
The `data.stalwart_dns_records` data source reads that field.

## Collection encoding

**Collection-valued properties are JSON objects, not arrays.**
Sending an array is rejected with `invalidPatch: Invalid value for object property`.

Two encodings are used, verified in the Rust serializers:

### `Map<T>` → `{"<value>": true, ...}`

A set keyed by the value, each mapped to `true`.

VERIFIED: `crates/registry/src/types/map.rs:222` (`map.serialize_entry(&item.as_string(), &true)`).

Used by: Domain `aliases`; Account/Group `memberGroupIds`; MailingList `recipients`; Role `roleIds`/`enabledPermissions`/`disabledPermissions`; DkimSignature `headers`; nested `roles.roleIds`, permission lists; Domain `certificateManagement.subjectAlternativeNames`.

**`Map<T>` is ordered.**
It is a `Vec<T>` behind the object encoding: `push()` appends after a containment check, nothing sorts, and deserialization pushes keys in document order.
So element order round-trips through the server intact rather than being normalised.

In Go: modelled as `StringSet` or `OrderedStringSet` (`internal/client/collections.go`), both marshalling empty as `{}` (required-present on create).
Pick between them by whether the *server* acts on order:

- **`StringSet` + `types.Set`** — the default.
  For fields where order carries no meaning (aliases, permissions, recipients), set semantics avoid spurious diffs when the server returns members in a different order than config lists them.
  `StringSet` marshals via a Go map and sorts on unmarshal, so order is deliberately discarded in both directions.
- **`OrderedStringSet` + `types.List`** — for the rare field whose order the server acts on.
  Today that is `certificateManagement.subjectAlternativeNames`: Stalwart builds the ACME order from it in order and submits an empty Subject, so the first entry becomes the issued certificate's Subject Common Name.

> **Do not marshal an order-significant field through `map[string]bool`.**
> `encoding/json` sorts map keys, so the order reaching the server is always alphabetical regardless of config — silently changing which hostname a certificate is issued for.
> This is why `OrderedStringSet` builds its JSON object explicitly.

See [ADR 003](decisions/003-ordered-map-collections.md) for the full rationale, including why set semantics remain the default.

### `List<T>` → `{"0": item, "1": item, ...}`

Keyed by stringified index.

VERIFIED: `crates/registry/src/types/list.rs:168` (`map.serialize_entry(&key.to_string(), value)`).

Used by: Account/Group `credentials` and `aliases` (`List<EmailAlias>`).

In Go: modelled as `IndexList[T]` (`internal/client/collections.go`).
Marshals empty as `{}`.

**Model as `types.List`** — `List<T>` fields are genuinely ordered.

### `VecMap` → plain object

`quotas` is `VecMap<StorageQuota,u64>` → `{"maxDiskQuota": 123}` (`crates/utils/src/map/vec_map.rs`).

### Naming collision

`aliases` is `Map<String>` on **Domain** (`structs.rs:2658`) but `List<EmailAlias>` on **Account/MailingList** — same JSON field name, different wire type.

## Optional + Computed attributes

**The server applies defaults and always returns collection fields** → Terraform attributes for such fields must be `Optional + Computed` (not bare `Optional`).
A bare `Optional` attribute that the server defaults triggers "Provider produced inconsistent result after apply" (was null, now server value).

Examples:

- `reportAddressUri` defaults to `"mailto:postmaster"` (`structs_impl.rs:19512`).
- Collection fields are non-`Option` in the Rust structs, so the server always returns them (as `{}` → empty collection on read).

Such attributes use `Optional + Computed` with a `UseStateForUnknown` plan modifier.

## Password strength

**Account passwords are strength-checked with zxcvbn.**
A weak password is rejected at create/update:

```text
invalidProperties: Password is too weak ... (properties: [secret])
```

VERIFIED: `crates/common/src/network/security.rs` (`zxcvbn::zxcvbn`, `password_min_strength`).
Acceptance tests use an uncommon multi-word passphrase.

## Domain name validation

**Domain names must have a recognised TLD.**
`is_valid_domain` (`crates/utils/src/lib.rs:356`) accepts a name only if its TLD is in the public suffix list or is one of: `test`, `localhost`, `local`, `internal`.

`.example` is rejected (`invalidPatch: Invalid domain name`).
Acceptance tests use `*.test`.

## Permission values

**Permissions are camelCase JMAP identifiers** (e.g. `emailSend`, `emailReceive`, `impersonate`), not kebab-case.
Invalid values are rejected: `invalidPatch: Invalid key for object property`.

VERIFIED: `crates/registry/src/schema/enums_impl.rs` (`Permission::EmailSend => "emailSend"`).
The full list is the `Permission` enum in `crates/registry/src/schema/enums.rs`.

## Duration fields

**Duration fields are `u64` milliseconds on the wire**, not a string.
Sending `"90d"` is rejected (`invalidPatch: Invalid path for Duration`).

VERIFIED: `crates/registry/src/types/duration.rs` (serialize/deserialize as `u64` millis).

The provider accepts the friendly string form (`90d`, `1h`, `500ms`; units d/h/m/s/ms, no unit = ms) and converts using `parseDuration`/`formatDuration`.
Applies to: dkim `expiry` (`expire` on the wire).
