# Architecture

## Overview

This is a Terraform provider for [Stalwart Mail Server](https://stalw.art) built on the **Terraform Plugin Framework** (not the legacy SDK). It targets Stalwart v0.16+, which exposes all management configuration as JMAP objects — there is no REST management API.

## Package layout

```
internal/
├── acctest/      Container harness for acceptance tests
│   ├── container.go   Spins up Stalwart in Docker, wires the provider config
│   └── ready.go       Health-check polling
├── client/       Minimal JMAP client
│   ├── client.go      HTTP transport, Basic/Bearer auth
│   ├── collections.go StringSet and IndexList types (Map<T> / List<T> wire encoding)
│   ├── id.go          IsID — distinguishes opaque server ids from human refs
│   ├── jmap.go        JMAP request/response envelope
│   ├── methods.go     Get / Set / Query typed helpers
│   └── objects.go     Go structs for each Stalwart object type
└── provider/     Terraform resources and data sources
    ├── provider.go    Provider registration, authentication configuration
    ├── shared.go      Shared schema helpers (domain_ref attribute, etc.)
    ├── planmodifiers.go  UseStateForUnknown and related plan modifiers
    ├── *_resource.go  One file per resource
    ├── *_data_source.go  One file per data source
    └── *_test.go      Full-lifecycle acceptance tests
```

## Client design

The `internal/client` package is intentionally minimal — it does not generate Terraform schema or know about provider configuration. Its responsibilities are:

- **Transport**: `POST /jmap` with Basic or Bearer authentication.
- **Encoding**: `StringSet` (Go representation of `Map<T>`) and `IndexList[T]` (Go representation of `List<T>`). Both marshal to empty `{}` when empty, which is required on create. See [stalwart-api.md](stalwart-api.md#collection-encoding) for the wire format.
- **Id detection**: `client.IsID(s)` returns true when `s` is in the Stalwart base32 alphabet (`abcdefghijklmnopqrstuvwxyz792013`). Used during import and domain ref resolution.
- **Typed JMAP helpers**: `Get[T]`, `Set[T]`, `Query[T]` — generic wrappers that handle the JMAP envelope and decode typed responses.

The client does **not** use the Stalwart CLI (`stalwart-cli`). The CLI is not bundled in the server Docker image and is a separate versioned binary.

## Provider / resource patterns

Each resource follows this structure:

1. **Schema**: Attributes mirror the Stalwart object's JSON fields. Fields that the server defaults or always returns are `Optional + Computed` with `UseStateForUnknown`. `Map<T>`-backed fields use `types.Set`; `List<T>`-backed fields use `types.List`. Duration fields accept a friendly string (`90d`, `1h`, `500ms`) and convert to/from milliseconds.

2. **Create / Update**: Build the Go struct, call `client.Set` with a `create` or `update` map. For create, the server assigns the id.

3. **Read**: Call `client.Get` by id, map the response back to Terraform state. All reads go through the provider's own read path — acceptance tests use a *separate* client to verify the server state independently.

4. **Delete**: Call `client.Set` with a `destroy` list containing the id.

5. **Import**: Accept either an opaque id (recognized by `client.IsID`) or a human-friendly reference (name, email, description). Human refs use `x:Type/query` to resolve to the id before importing.

## Domain references

Several objects (dkim_signature, account, mailing_list, role) reference a domain. The `domain_ref` shared attribute accepts either the domain resource's id or its name. On read, the provider stores the id (stable across renames). On create, if a name is given, it resolves to an id via query.

## Plugin Framework specifics

- Use `diag.Diagnostics` for all error reporting; never `panic`.
- `UseStateForUnknown` plan modifier on `Optional + Computed` attributes prevents spurious diffs when the server echoes back a value the config omitted.
- Acceptance tests use `resource.TestCase` with `TF_ACC=1`; they require a live Stalwart instance (provided by the container harness in `internal/acctest`).
