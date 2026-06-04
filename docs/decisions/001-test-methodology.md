# 001 — Use direct JMAP client for server-side assertions in acceptance tests

**Status**: Accepted

## Context

Acceptance tests need to verify not just what the Terraform provider *reads back* from the server, but what the server *actually stored*. These are distinct: a write-path bug and a matching read-path bug can cancel each other out, making `TestCheckResourceAttr` pass while the server holds the wrong value.

An alternative approach considered was `stalwart-cli snapshot`: capture a JSON dump before and after each apply step and diff the results.

## Decision

Each acceptance test asserts field values **twice**:

1. In Terraform state via `resource.TestCheckResourceAttr` — proves plan/apply/read round-trips correctly.
2. On the server via a **direct JMAP client** (`accClient` in `acc_helpers_test.go`) built independently of the provider's read path. The `checkServer*` helpers in `acc_checks_test.go` fetch the object and assert exact field values.

The direct client also verifies id linkages: a child object's `domainId`/`memberGroupIds`/`roleIds` on the server must equal the ids that Terraform state recorded for the referenced resources.

## Rejected alternative: `stalwart-cli snapshot`

`stalwart-cli snapshot` was considered and rejected for three reasons:

1. **Extra dependency**: the CLI is not bundled in the `stalwartlabs/stalwart` Docker image. Using it in the harness requires installing a separate, version-matched binary.

2. **Lossy by design**: snapshot strips secrets and server-set fields, masks values, and rewrites ids to client-refs (e.g. `#domain-b`). It cannot assert the exact value of any field.

3. **Wrong tool**: snapshot is designed for backup and migration, not per-field verification. The direct-client read gives a more precise "exactly these fields are as expected" guarantee.

Revisit only if a future need (e.g. asserting the *absence* of unexpected objects across the whole server state) calls for a whole-state diff.

## Consequences

- `acc_checks_test.go` must be kept in sync with each resource's writable fields.
- Adding a new resource requires adding `checkServer*` helpers alongside the lifecycle test.
- The direct-client approach is more verbose but gives high confidence that the provider writes what it intends to.
