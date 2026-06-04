# Testing Guide

## Unit tests

```sh
go build ./... && go vet ./... && go test ./...
```

No external services required. Tests live alongside their source files (`*_test.go`) in `internal/client/` and `internal/provider/`.

## Acceptance tests

Acceptance tests exercise the full provider lifecycle against a live Stalwart instance. They require `TF_ACC=1` and are run via:

```sh
make testacc
```

The harness (`internal/acctest/`) starts a Stalwart container automatically — no user-provided instance or credentials required.

### Container image

Default: `stalwartlabs/stalwart:v0.16`. Override with `STALWART_TEST_IMAGE` for version matrix testing.

### How the container is configured

The harness starts Stalwart in **recovery mode**, which exposes the full JMAP management API over plain HTTP with no TLS, no setup wizard, and no mail services:

- `STALWART_RECOVERY_MODE=1` — disables background/mail services, serves only the management API.
- `STALWART_RECOVERY_ADMIN=admin:<password>` — pins a deterministic admin credential.
- `STALWART_RECOVERY_MODE_PORT` (default `8080`) — the HTTP management port.
- A minimal `config.json` is written containing only the DataStore object:
  `{"@type":"RocksDb","path":"/var/lib/stalwart/"}`.

The management API is reachable at `http://<host>:8080/jmap`.

**Recovery mode vs bootstrap mode**: without `config.json`, the first start enters bootstrap mode (random admin password in stderr, only the `Bootstrap` object exposed). Recovery mode is deterministic and preferred for tests.

### Test methodology

Each resource/data-source has a full-lifecycle acceptance test in `internal/provider/*_resource_test.go`. For every writable field, the value is asserted **twice**:

1. **In Terraform state** via `resource.TestCheckResourceAttr` — proves the provider round-trips the value through plan/apply/read with no inconsistency.
2. **On the server** via a direct JMAP client built independently of the provider's read path (`accClient` + `checkServer*` helpers in `acc_checks_test.go`).

The second assertion is the "don't grade your own homework" guarantee: a write-path bug cannot be masked by a matching read-path bug. It also verifies id linkages (a child's server-side `domainId`/`memberGroupIds`/`roleIds` equal the referenced resources' ids from Terraform state).

See [decisions/001-test-methodology.md](decisions/001-test-methodology.md) for the rationale behind this approach and why `stalwart-cli snapshot` was rejected.

### Test helpers

| File | Purpose |
|---|---|
| `acc_helpers_test.go` | `accClient` construction, HCL list rendering |
| `acc_checks_test.go` | Per-type server fetch + field assertion helpers (`wantStr`, `wantBool`, `wantSet`, `wantQuota`, ...) |

## CI

The `testacc` job runs on every PR and push to main via `.github/workflows/test.yml`. It tests against a matrix of Stalwart versions (currently `["v0.16"]`).

Coverage is enforced at a minimum floor (currently 68%). The CI job uploads a coverage report artifact and posts a comment on PRs when coverage changes.

**In the Claude Code web environment, run acceptance tests via CI, not locally.** The environment cannot pull the Stalwart container image due to network restrictions. See [decisions/002-ci-over-local.md](decisions/002-ci-over-local.md) for details and how to check whether the restriction has been lifted.

## Linting and formatting

```sh
gofmt -l -s .          # must print nothing
golangci-lint run ./... # requires golangci-lint v2.x
```

`.golangci.yml` uses the **v2 config format** (`version: "2"`), which requires golangci-lint v2.x. The older `@v6` GitHub Action installs v1.x and fails with exit code 3 (config load error). CI pins `golangci-lint-action@v7` with `version: v2.5.0`.

The `errcheck` linter requires explicitly discarding deferred `Close()` errors:

```go
defer func() { _ = x.Close() }()
```
