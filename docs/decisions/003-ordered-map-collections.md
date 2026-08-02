# 003 — Model order-significant `Map<T>` fields as `types.List`

**Status**: Accepted

## Context

Stalwart encodes collection-valued properties as JSON objects rather than arrays.
Its `Map<T>` renders as `{"<value>": true, ...}`, which reads like an unordered set — and the provider modelled every such field as `types.Set`, backed by a `StringSet` that marshalled through `map[string]bool` and sorted on unmarshal.

That modelling rested on a claim recorded in [stalwart-api.md](../stalwart-api.md#collection-encoding): that the server returns `Map<T>` fields in canonical sorted order, so a `types.List` would produce "inconsistent result after apply" whenever config order differed.

The claim was wrong.
`Map<T>` is a `Vec<T>` behind the object encoding (upstream `crates/registry/src/types/map.rs`): `push()` appends after a containment check, nothing sorts, and deserialization pushes keys in document order.
Element order round-trips through the server intact.
The sorting the provider observed was its own — `encoding/json` sorts map keys, so every write left alphabetically ordered regardless of configuration.

This is inert for most fields, where order carries no meaning.
It is silently destructive for `certificateManagement.subjectAlternativeNames`: Stalwart builds the ACME order from that collection in order and submits an empty Subject, leaving the CA to name the certificate after the first entry.
A domain configured with `mail`, `autoconfig`, `autodiscover` was written alphabetically and issued a certificate whose Common Name was `autoconfig.<domain>` — while still carrying `mail.<domain>` as a SAN, so TLS validation passes and only CN-displaying clients ever surface the discrepancy.

## Decision

`Map<T>` fields are modelled by whether **the server acts on element order**, not by their wire format:

| Server behaviour | Go type | Schema type |
| --- | --- | --- |
| Order carries no meaning (aliases, permissions, recipients) | `StringSet` | `types.Set` |
| Server acts on element position | `OrderedStringSet` | `types.List` |

`OrderedStringSet` shares the wire format but builds its JSON object explicitly — never via a Go map — and decodes by token stream in document order.
Duplicates are dropped keeping the first occurrence, mirroring the server's own `push()`.

Today `certificateManagement.subjectAlternativeNames` is the only field in the second category.

Set semantics remain the default deliberately: they avoid spurious diffs when the server returns members in a different order than config lists them, which is the right behaviour everywhere order is not load-bearing.

## Consequences

- `subject_alternative_names` is a list, so a reorder alone plans an update. This is a breaking schema change; HCL list literals parse unchanged and existing state decodes without a migration, since set and list both encode as JSON arrays in state.
- The schema version is deliberately **not** bumped and no state upgrader is provided: the transformation is a no-op, so an upgrader would have nothing to do. The first plan after upgrading may show a reorder diff, which is the intended correction rather than a migration artifact.
- Adding a `Map<T>`-backed field requires deciding which category it falls into. Getting it wrong is silent in both directions: a set where order matters corrupts the value, a list where it does not produces perpetual diffs.
- `TestOrderedStringSetVsStringSet` pins the behavioural difference between the two types so a refactor collapsing them fails loudly rather than quietly reintroducing the sort.
- Any future field whose order the server acts on must not be marshalled through `map[string]bool`, regardless of how convenient it looks.
