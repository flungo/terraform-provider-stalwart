# 004 — Support Stalwart v0.16 and v1 from one provider release

**Status**: Accepted

## Context

Stalwart v1.0 changed the management schema the provider writes.
It is not an in-place upgrade: a v0.16 deployment moves to v1 by a data migration with `stalwart-migrate`.
Consumers migrating from 0.16 want Terraform usable at every point of that move: bump the provider while the server is still on 0.16, migrate the server, then see a no-diff `terraform plan` straight after.
A v1-only provider would force the provider bump and the server cutover into one lockstep change, and would break any CI that applies a configuration against a v0.16 server on every push to `main`.

So the question was whether one provider build can talk to both versions, and at what cost.

### What changed for this provider

Upstream records the full v0.16.27 → v1.0.3 diff in `SCHEMA_CHANGES.md` of [stalwart-migrate](https://github.com/stalwartlabs/stalwart-migrate).
Every object, sub-object, enum and query filter this provider touches was diffed property by property between the schema snapshots it ships (`mapping/schema/v0.16.27.json.gz`, `mapping/schema/v1.json.gz`).
Those snapshots are byte-identical to the `resources/schema/schema.json.gz` the server embeds at tags `v0.16.27` and `v1.0.3`, which is the document `/api/schema` serves.

| Object | v0.16 | v1 | Provider attribute |
| --- | --- | --- | --- |
| `Domain` | `isEnabled` | `enable` | `enabled` |
| `DnsServer` (every variant) | `pollingInterval` | `pollInterval` | `polling_interval` |
| `NetworkListener` | `tlsImplicit` | `implicitTls` | `tls_implicit` |
| `LdapDirectory` | `filterMailbox` | `filterRecipient` | `filter_mailbox` |
| `AcmeProvider` | `maxRetries` | removed | `max_retries` |
| `AcmeProvider` | `description` server-set (derived) | `description` stored and required | `description` (new; defaults to the directory URL on v1) |
| `OidcDirectory` | `requireScopes` | removed | `require_scopes` |
| `OidcDirectory` | — | `acceptedAudiences`, `unscopedTokens`, `opaqueTokenValidation`, `skipAudienceValidation`, `allowIdTokenClaims`, `lenientIssuerMatch`, `allowInvalidCerts` | new attributes |

Everything else the provider uses is unchanged: the JMAP endpoint, `x:` method names, collection encodings, object ids, the query filters (`name`, `text`, `description`, `domainId`), and every enum value it sends (`AcmeChallengeType`, `AcmeRenewBefore`, `NetworkListenerProtocol`, `DkimCanonicalization`, `DnsRecordType`, `TsigAlgorithm`, `StorageQuota`, the `@type` variants of `Account`, `Credential`, `UserRoles`, `Permissions`, `EncryptionAtRest.Disabled`, `SecretKey`, `SecretText`, `DkimSignature.Dkim1*`, the management groups of `Domain`).
v1 renumbered ids and enum positions, but the JMAP API carries names, never positions, so that does not reach the wire.
The `Permission` vocabulary changed (37 names removed, 38 added), but the role, account and group resources pass permission names through, so a configuration that names a permission the server lacks is rejected by the server on either version.

The per-version surface is therefore four renames, two removals and a handful of additions — small and stable within each series.

## Decision

Support both versions in one release.

**Detection.**
The client learns the server's schema generation on first use by fetching `GET /api/schema` (authenticated; it redirects to `/api/schema/<sha256>` and serves gzip) and checking whether `fields["x:Domain"].properties` has `enable` (v1) or `isEnabled` (v0.16).
That is the server's own description of the properties it validates, so it classifies by the very difference that matters, and it exists in every release from v0.16.0 on.
The result is cached per client; a failed detection is not cached.
A provider setting, `server_version` (`0.16` or `1`, or `STALWART_SERVER_VERSION`), skips detection for deployments that expose only `/jmap`.

Rejected alternatives:

- *A version string from the server.*
  None is exposed over HTTP: the JMAP session's `implementation` string is the hard-coded `"Stalwart v1.0.0"` in both series, and `VERSION_PUBLIC` is `"1.0.0"` in both.
- *A capability in the JMAP session.*
  v1 adds `urn:ietf:params:jmap:metadata`, but its presence is a feature flag rather than a statement about the management schema.
- *Probing with `x:Domain/get` and a `properties` list.*
  The server drops property names it does not know instead of rejecting them, and both versions know `enable` as a property name of other objects.

**Wire names.**
The Go structs in `internal/client` use the v1 property names.
A table in `internal/client/version.go` lists the v0.16 name of each renamed property per object type; on a v0.16 server the client renames those keys in `set` payloads (including the first segment of a patch path) and in `get` results.
Resources never see the v0.16 names, and adding v1 support required no change to any resource whose objects were only renamed.

**Version-specific attributes.**
Attributes whose property exists on only one version are `Optional + Computed`.
Setting one in configuration against the other version is a plan-time error naming the attribute and the version, raised from `ModifyPlan` (and re-checked at apply for values unknown at plan).
Reads leave the attribute null (or an empty set) on the version that lacks it, and the wire object only ever carries the property to the version that has it, so a value held in state from before a migration is never sent to the wrong version.
Terraform attribute names do not change: `filter_mailbox` and `tls_implicit` keep their v0.16-era names even though v1 calls the properties `filterRecipient` and `implicitTls`, so a configuration needs no edit to move between versions.

## Consequences

- One release serves both series, so the provider pin can move ahead of the server migration, and a CI job applying against a v0.16 server keeps working.
- After `stalwart-migrate`, a plan shows no diff for configurations that use only common attributes: refresh reads the same values under the new wire names into unchanged attribute names.
  The one exception is a configuration that sets a v0.16-only attribute (`max_retries`, `require_scopes`); it fails at plan time against v1 with an error saying to remove it.
  Removing it while still on v0.16 is itself a no-op plan, because both are `Optional + Computed` and keep their state value.
- Detection costs one authenticated request for a ~150–180 KB gzip document per provider process, made only when a translated object or a version-specific attribute is first touched.
  Credentials that can call `/jmap` can also fetch `/api/schema`; it needs authentication, not a permission.
- Acceptance tests run against both `v0.16.27` and `v1.0.3` as a CI matrix.
  `TestAccServerVersionDetection` asserts that detection agrees with the image tag, and `TestAccWireNames` reads a domain through a client that does not translate, so the rename table itself is checked against the server rather than only round-tripped through it.
- Extending the provider means checking each new property against both schemas.
  A rename goes in the table; a property on only one version becomes a `versionedAttr`.
- v0.16 support should be dropped once consumers have migrated: delete the table, the `Version016` branches and the matrix entry, and turn the v0.16-only attributes into a breaking removal in a major release.
