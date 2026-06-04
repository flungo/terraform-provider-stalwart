# CLAUDE.md

Operational rules and quick-reference for AI coding sessions on this repository.
Detailed reference lives in [`docs/`](docs/README.md) — read it when implementing or extending resources.

## Project shape

- Terraform provider on the **Plugin Framework** (not the legacy SDK).
- Targets **Stalwart v0.16+**, which exposes all configuration as JMAP objects (no REST management API).
- `internal/client/` — minimal JMAP client. `internal/provider/` — resources and data sources.
- See [docs/architecture.md](docs/architecture.md) for package layout and design patterns.

## Branch workflow

**Always work on a feature branch; never commit directly to `main`.**

- **At the start of a session, establish the branch:**
  - If `main` is checked out: `git fetch origin && git pull --ff-only origin main`, then create the feature branch off it.
  - If another (non-`main`) branch is already checked out: confirm with the user that they want to continue on this branch before doing anything else.
- **After each prompt, fetch and rebase** onto the latest `main`:
  `git fetch origin && git rebase origin/main`. If upstream changed, review what changed, judge whether it affects this branch, and make required adjustments before continuing.
- **When rebasing and fixing issues, fold the fixes into the relevant existing commits** (amend during rebase). The linear history must read as though those changes were always present — never append a "fix-up" commit for an upstream change.

## Stalwart API — critical facts

Full reference: [docs/stalwart-api.md](docs/stalwart-api.md). The most common traps:

- **JMAP endpoint is `/jmap`**, not `/api`. The generated `ref/object/*.md` curl snippets are wrong.
- **Wire method names carry `x:` prefix**: `x:Domain/get`, `x:Account/set`, etc.
- **Collections are JSON objects, not arrays.** `Map<T>` → `{"value": true}` (use `types.Set`). `List<T>` → `{"0": item, "1": item}` (use `types.List`).
- **Server defaults and always-returns collections** → attributes must be `Optional + Computed` with `UseStateForUnknown`.
- **Duration fields are `u64` milliseconds** on the wire. The provider converts from/to friendly strings (`90d`, `1h`, `500ms`).
- **Domain names need a recognised TLD.** Use `*.test` in acceptance tests, not `*.example`.

## Source verification

When extending the provider, verify facts against the upstream source — do NOT trust the generated docs:

```sh
git clone --depth 1 https://github.com/stalwartlabs/stalwart.git /tmp/stalwart-src
git clone --depth 1 https://github.com/stalwartlabs/website.git    /tmp/website
# Object schemas:
ls /tmp/website/src/content/docs/docs/ref/object/
```

Key paths in `stalwart-src`:
- Object schemas: `crates/registry/src/schema/structs.rs`
- Collection wire formats: `crates/registry/src/types/{map,list}.rs`
- HTTP routing: `crates/http/src/request.rs`
- Method/capability naming: `crates/jmap-proto/src/request/{capability,method}.rs`

## Acceptance tests — CI only (web environment)

**Do not attempt to run acceptance tests locally in the Claude Code web environment.** Container image pulls are blocked by network restrictions. Push to the feature branch and iterate against the CI `testacc` job in GitHub Actions.

See [docs/decisions/002-ci-over-local.md](docs/decisions/002-ci-over-local.md) for the full context and instructions for checking whether the restriction has been lifted.

## Verification commands

```sh
go build ./... && go vet ./... && go test ./...   # unit tests, no network
gofmt -l -s .                                      # must print nothing
golangci-lint run ./...                            # needs v2.x
make testacc                                       # acceptance (needs container — CI only in web env)
```

## Tooling gotchas

- `.golangci.yml` uses **golangci-lint v2 config format** — requires v2.x. The `@v6` GitHub Action installs v1.x and fails with exit code 3.
- The `errcheck` linter requires explicitly discarding deferred `Close()` errors: `defer func() { _ = x.Close() }()`.
- Go toolchain auto-upgrades in this environment (`go.mod` shows `go 1.25.8`).
