# Stalwart API Reference

Corrected facts about the Stalwart v0.16+ JMAP management API. Every claim here has been verified against the upstream source code and is noted with the relevant file path.

**Upstream sources** (fact-checked at these commits):
- Code: `stalwartlabs/stalwart` @ `68946d4` (2026-05-29)
- Docs: `stalwartlabs/website` @ `8044db7` (2026-05-29)

Clone these when extending the provider — do not rely on the generated `ref/object/*.md` curl snippets (they show a wrong endpoint):

```sh
git clone --depth 1 https://github.com/stalwartlabs/stalwart.git /tmp/stalwart-src
git clone --depth 1 https://github.com/stalwartlabs/website.git    /tmp/website
# Object schemas:
ls /tmp/website/src/content/docs/docs/ref/object/
```

The official docs site (`stalw.art/docs`) blocks automated fetches (HTTP 403). The GitHub raw API rate-limits unauthenticated requests — prefer a shallow `git clone`.

## JMAP endpoint

**The endpoint is `/jmap`, not `/api`.**

- `crates/http/src/request.rs` routes `"jmap" =>` (POST, empty next segment) to `handle_jmap_request` (~line 83).
- `"api" =>` routes to a separate `handle_api_request` (~line 403) — a small HTTP API for auth, schema, and telemetry. It is NOT the management API.
- Empirically confirmed: `POST /api` → **404**, `POST /jmap` → **200** on a live v0.16 container.
- The generated `ref/object/*.md` curl snippets show `POST .../api` and are **wrong**. The hand-written `http/index.md` and `development/api.md` are correct.

## Capability URN

`urn:stalwart:jmap` (plus `urn:ietf:params:jmap:core`). Not `urn:stalwart:core`.

VERIFIED: `crates/jmap-proto/src/request/capability.rs` (`Capability::Stalwart => "urn:stalwart:jmap"`).

## Wire method names

Method names on the wire carry an **`x:` prefix**: `x:Domain/get`, `x:Account/set`, `x:DkimSignature/query`, etc.

VERIFIED: `crates/jmap-proto/src/request/method.rs` (`format!("x:{}/{}", obj, method)` and `s.strip_prefix("x:")`). The CLI omits the prefix in display output but it is required on the wire.

Standard JMAP semantics: `Foo/get`, `Foo/set` (create/update/destroy), `Foo/query`. Singletons use the literal id `"singleton"`.

## Object ids

Objects are keyed by an **opaque server-generated id** (`id` field). Child objects reference their parent via a field like `domainId` — the parent's id, not its name.

**The id is NOT a ULID.** It is a `u64` rendered in a custom base32 alphabet:

```
abcdefghijklmnopqrstuvwxyz792013
```

1–13 lowercase characters. Id `0` → `"a"`.

VERIFIED: `crates/types/src/id.rs` + `crates/utils/src/codec/base32_custom.rs`.

`client.IsID(s)` in this provider recognises this alphabet to distinguish an opaque id from a human-friendly reference (name, email, description). Human refs always contain `.`, `@`, space, or an uppercase letter — none of which are in the alphabet.

## Accounts and groups

**Accounts and groups are the same `Account` object**, discriminated by `@type`:
- `"User"` → account
- `"Group"` → group

There is no separate Group JMAP object type.

## DNS records

DNS recommendations are **not a separate JMAP method**. They are the read-only `dnsZoneFile` text field on the `Domain` object. The `data.stalwart_dns_records` data source reads that field.

## Collection encoding

**Collection-valued properties are JSON objects, not arrays.** Sending an array is rejected with `invalidPatch: Invalid value for object property`.

Two encodings are used, verified in the Rust serializers:

### `Map<T>` → `{"<value>": true, ...}`

A set keyed by the value, each mapped to `true`.

VERIFIED: `crates/registry/src/types/map.rs:222` (`map.serialize_entry(&item.as_string(), &true)`).

Used by: Domain `aliases`; Account/Group `memberGroupIds`; MailingList `recipients`; Role `roleIds`/`enabledPermissions`/`disabledPermissions`; DkimSignature `headers`; nested `roles.roleIds`, permission lists.

In Go: modelled as `StringSet` (`internal/client/collections.go`). Marshals empty as `{}` (required-present on create).

**Model as `types.Set` in Terraform schema** — the server returns `Map<T>` fields in canonical (sorted) order, so `types.List` would produce "inconsistent result after apply" whenever config order differs from server order.

### `List<T>` → `{"0": item, "1": item, ...}`

Keyed by stringified index.

VERIFIED: `crates/registry/src/types/list.rs:168` (`map.serialize_entry(&key.to_string(), value)`).

Used by: Account/Group `credentials` and `aliases` (`List<EmailAlias>`).

In Go: modelled as `IndexList[T]` (`internal/client/collections.go`). Marshals empty as `{}`.

**Model as `types.List`** — `List<T>` fields are genuinely ordered.

### `VecMap` → plain object

`quotas` is `VecMap<StorageQuota,u64>` → `{"maxDiskQuota": 123}` (`crates/utils/src/map/vec_map.rs`).

### Naming collision

`aliases` is `Map<String>` on **Domain** (`structs.rs:2658`) but `List<EmailAlias>` on **Account/MailingList** — same JSON field name, different wire type.

## Optional + Computed attributes

**The server applies defaults and always returns collection fields** → Terraform attributes for such fields must be `Optional + Computed` (not bare `Optional`). A bare `Optional` attribute that the server defaults triggers "Provider produced inconsistent result after apply" (was null, now server value).

Examples:
- `reportAddressUri` defaults to `"mailto:postmaster"` (`structs_impl.rs:19512`).
- Collection fields are non-`Option` in the Rust structs, so the server always returns them (as `{}` → empty collection on read).

Such attributes use `Optional + Computed` with a `UseStateForUnknown` plan modifier.

## Password strength

**Account passwords are strength-checked with zxcvbn.** A weak password is rejected at create/update:

```
invalidProperties: Password is too weak ... (properties: [secret])
```

VERIFIED: `crates/common/src/network/security.rs` (`zxcvbn::zxcvbn`, `password_min_strength`). Acceptance tests use an uncommon multi-word passphrase.

## Domain name validation

**Domain names must have a recognised TLD.** `is_valid_domain` (`crates/utils/src/lib.rs:356`) accepts a name only if its TLD is in the public suffix list or is one of: `test`, `localhost`, `local`, `internal`.

`.example` is rejected (`invalidPatch: Invalid domain name`). Acceptance tests use `*.test`.

## Permission values

**Permissions are camelCase JMAP identifiers** (e.g. `emailSend`, `emailReceive`, `impersonate`), not kebab-case. Invalid values are rejected: `invalidPatch: Invalid key for object property`.

VERIFIED: `crates/registry/src/schema/enums_impl.rs` (`Permission::EmailSend => "emailSend"`). The full list is the `Permission` enum in `crates/registry/src/schema/enums.rs`.

## Duration fields

**Duration fields are `u64` milliseconds on the wire**, not a string. Sending `"90d"` is rejected (`invalidPatch: Invalid path for Duration`).

VERIFIED: `crates/registry/src/types/duration.rs` (serialize/deserialize as `u64` millis).

The provider accepts the friendly string form (`90d`, `1h`, `500ms`; units d/h/m/s/ms, no unit = ms) and converts using `parseDuration`/`formatDuration`. Applies to: dkim `expiry` (`expire` on the wire).
