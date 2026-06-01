# CLAUDE.md

Working notes for developing the Stalwart Terraform provider. Records hard-won
facts about the Stalwart API and the test environment so future sessions can
start fast.

## Project shape

- Terraform provider on the **Plugin Framework** (not the legacy SDK).
- Targets **Stalwart v0.16+**, which dropped the REST management API and exposes
  all configuration as JMAP objects.
- `internal/client/` — minimal JMAP client. `internal/provider/` — resources and
  data sources.

## Source verification

Every non-obvious claim below has been fact-checked against the two upstream
repos. Clone both and grep them when extending the provider; do NOT trust the
generated `ref/object/*.md` curl snippets for the endpoint path (they are wrong
— see below).

- Code: `stalwartlabs/stalwart` — verified at commit `68946d4` (2026-05-29).
- Docs: `stalwartlabs/website` — verified at commit `8044db7` (2026-05-29).

```sh
git clone --depth 1 https://github.com/stalwartlabs/stalwart.git /tmp/stalwart-src
git clone --depth 1 https://github.com/stalwartlabs/website.git    /tmp/website
```

The management object schema lives in
`stalwart-src/crates/registry/src/schema/structs.rs` (one Rust struct per
object, with `#[serde(rename = "...")]` giving the JSON field names). Collection
wire formats are in `crates/registry/src/types/{map,list}.rs`. HTTP routing is in
`crates/http/src/request.rs`. JMAP capability/method naming is in
`crates/jmap-proto/src/request/{capability,method}.rs`.

## Stalwart management API (corrected against the real schema)

The original task brief had several inaccuracies; the truth, fact-checked
against code (`stalwart-src`) and docs (`website`):

- **JMAP endpoint is `/jmap`** (the client appends it to the configured base
  endpoint). VERIFIED in code: `crates/http/src/request.rs` routes `"jmap" =>`
  (empty next segment + POST) to `handle_jmap_request` (~line 83), while
  `"api" =>` routes to a separate `handle_api_request` (~line 403). Also
  confirmed empirically against a live v0.16 container: `POST /api` → **404**,
  `POST /jmap` → **200** for the same JMAP body. The generated
  `ref/object/*.md` curl snippets show `POST .../api` and are WRONG; the
  hand-written `http/index.md` / `development/api.md` correctly state JMAP is at
  `/jmap` and `/api/*` is a small separate HTTP API (auth, schema, telemetry).
- **Capability URN is `urn:stalwart:jmap`** (+ `urn:ietf:params:jmap:core`), not
  `urn:stalwart:core`. VERIFIED: `crates/jmap-proto/src/request/capability.rs`
  (`Capability::Stalwart => "urn:stalwart:jmap"`).
- **Wire method/type names carry an `x:` prefix**: `x:Domain/get`,
  `x:Account/set`, `x:DkimSignature/query`, etc. VERIFIED:
  `crates/jmap-proto/src/request/method.rs` (`format!("x:{}/{}", obj, method)`
  and `s.strip_prefix("x:")`). The CLI omits the prefix for display but it is
  required on the wire.
- **Accounts and groups are the same `Account` object**, discriminated by an
  `@type` of `"User"` vs `"Group"`. There is no separate Group object.
- **DNS recommendations are not a separate method**: they are the read-only
  `dnsZoneFile` text field on the `Domain` object. `data.stalwart_dns_records`
  reads that field.
- Objects are keyed by an **opaque server-generated ULID** (`id`). Child objects
  reference their domain via `domainId` (the ULID), not the domain name.
- Standard JMAP `Foo/get` | `Foo/set` (create/update/destroy) | `Foo/query`
  semantics throughout. Singletons use the literal id `singleton`.
- **Collection-valued properties are JSON objects, NOT arrays.** Sending an array
  is rejected with `invalidPatch: Invalid value for object property`. Two
  encodings, VERIFIED in the Rust serializers:
  - `Map<T>`  -> `{"<value>": true, ...}` (a set keyed by value).
    VERIFIED: `crates/registry/src/types/map.rs:222`
    (`map.serialize_entry(&item.as_string(), &true)`). Used by: Domain
    `aliases`; Account/Group `memberGroupIds`; MailingList `recipients`; Role
    `roleIds`/`enabledPermissions`/`disabledPermissions`; DkimSignature
    `headers`; nested `roles.roleIds`, permission lists.
  - `List<T>` -> `{"0": <item>, "1": <item>}` (keyed by stringified index).
    VERIFIED: `crates/registry/src/types/list.rs:168`
    (`map.serialize_entry(&key.to_string(), value)`). Used by: Account/Group
    `credentials` and `aliases` (`List<EmailAlias>`).
  - `quotas` is `VecMap<StorageQuota,u64>` -> plain `{"maxDiskQuota": 123}`
    (`crates/utils/src/map/vec_map.rs`).
  The client models these with `StringSet` / `IndexList[T]` (internal/client/
  collections.go), which marshal empty as `{}` (required-present on create).
  NOTE: `aliases` is `Map<String>` on Domain (`structs.rs:2658`) but
  `List<EmailAlias>` on Account/MailingList — same name, different wire type.
- **Server applies defaults / always-returns collections → Terraform attributes
  must be `Optional + Computed`.** A bare `Optional` attribute that the server
  defaults or echoes back triggers "Provider produced inconsistent result after
  apply" (was null, now <server value>). Examples VERIFIED:
  `reportAddressUri` default `"mailto:postmaster"`
  (`structs_impl.rs:19512`; docs `domain.md:121`); collection fields are
  non-`Option` in the structs so the server always returns them (as `{}` → an
  empty list on read). Such attributes use `Optional + Computed` with a
  `UseStateForUnknown` plan modifier in this provider.
- **`Map<T>` fields are unordered sets → model them as `types.Set`, not
  `types.List`.** The server stores `Map<T>` as a set and returns it in a
  canonical (sorted) order, so a `types.List` attribute trips "inconsistent
  result after apply" whenever config order ≠ server order (seen with role
  `enabledPermissions`: config `[emailSend, emailReceive]`, read back
  `[emailReceive, emailSend]`). Every `Map<T>`-backed attribute is a
  `SetAttribute`: domain `aliases`; account/group `member_of`/`role_ids`;
  mailing_list `recipients`; role `extends`/`enabled_permissions`/
  `disabled_permissions`; dkim `headers`. (`List<T>` fields like account
  `credentials` are genuinely ordered and stay lists.)
- **Domain names must have a real/recognised TLD.** `is_valid_domain`
  (`stalwart-src/crates/utils/src/lib.rs:356`) accepts a name only if its TLD is
  a public-suffix-list entry OR one of the reserved TLDs `test`, `localhost`,
  `local`, `internal`. `.example` is rejected (`invalidPatch: Invalid domain
  name`); acceptance tests use `*.test`.
- **Permission values are camelCase JMAP identifiers** (e.g. `emailSend`,
  `emailReceive`), not kebab-case. VERIFIED:
  `crates/registry/src/schema/enums_impl.rs` (`Permission::EmailSend =>
  "emailSend"`).

### Reference docs

The official docs site (`stalw.art/docs`) **blocks automated fetches (HTTP 403)**
and DeepWiki rate-limits aggressively. Instead, clone the docs source and read
the Markdown directly:

```sh
git clone --depth 1 https://github.com/stalwartlabs/website.git /tmp/website
# Object schemas (fields + JMAP methods + curl examples + CLI examples):
ls /tmp/website/src/content/docs/docs/ref/object/
# e.g. domain.md, account.md, dkim-signature.md, mailing-list.md, role.md
```

The GitHub raw API (`api.github.com/.../git/trees`) rate-limits unauthenticated
requests quickly — prefer a shallow `git clone`.

## Acceptance test harness

`make testacc` spins up a real Stalwart instance in a container, points the
provider at it, and runs the `TF_ACC` tests — no user-provided instance or env
vars required. Image: `stalwartlabs/stalwart:v0.16` (overridable via
`STALWART_TEST_IMAGE` for version matrix testing). VERIFIED green in CI as of
the domain lifecycle test; the `testacc` job runs on every PR/push.

### How to bring up a *headless* Stalwart for testing

The key insight: **recovery mode** exposes the full JMAP management API over
plain HTTP with no TLS, no setup wizard, and no mail services — exactly what the
provider talks to.

- Write a minimal `config.json` containing only the DataStore object:
  `{"@type":"RocksDb","path":"/var/lib/stalwart/"}` (mount at
  `/etc/stalwart/config.json` / wherever `--config` points).
- Start the container with:
  - `STALWART_RECOVERY_MODE=1` — disables all background/mail services, serves
    only the HTTP management API.
  - `STALWART_RECOVERY_ADMIN=admin:<password>` — pins a known admin credential
    (no need to scrape the random bootstrap password from logs).
  - `STALWART_RECOVERY_MODE_PORT` (default `8080`) — the HTTP management port.
- The management API is then reachable at `http://<host>:8080/jmap`. The provider
  authenticates with HTTP Basic (`admin:<password>`) or a bearer token.
- Without `config.json`, the first start instead enters **bootstrap mode**
  (random admin password printed once to stderr, only the `Bootstrap` object
  exposed) — avoid for tests; recovery mode is deterministic.

`stalwart-cli apply <plan.json>` loads a batch of create/update/destroy ops in
dependency order — the intended way to apply fixtures. `stalwart-cli describe`
explores the schema (useful when extending the provider). The CLI is a separate
binary (`stalwartlabs/cli`), schema-driven; it fetches the schema from
`/api/schema` (the HTTP API) and issues JMAP method calls against `/jmap`.
**NOTE: `stalwart-cli` is NOT bundled in the `stalwartlabs/stalwart` image** —
the Dockerfile copies only the `stalwart` server binary. Using the CLI in the
harness would mean installing a separate, version-matched binary.

### Acceptance-test methodology (and why not `stalwart-cli snapshot`)

Each resource/data-source has a full-lifecycle acceptance test
(`internal/provider/*_resource_test.go`) that, for every writable field,
asserts the value **twice**:

1. In Terraform state, via `resource.TestCheckResourceAttr` (proves the provider
   round-trips the value through plan/apply/read with no inconsistency).
2. On the server, via a **direct JMAP client built independently of the
   provider's read path** (`accClient` + `checkServer*` helpers in
   `acc_checks_test.go`). This is the "don't grade your own homework" guarantee:
   a write-path bug can't be masked by a matching read-path bug. The check also
   verifies id linkages (a child's server-side `domainId`/`memberGroupIds`/
   `roleIds` equal the referenced resources' ids from state).

Common behaviour is abstracted in `acc_helpers_test.go` (client construction,
HCL list rendering) and `acc_checks_test.go` (per-type server fetch + field
assertion helpers: `wantStr`, `wantBool`, `wantSet`, `wantQuota`, ...).

`stalwart-cli snapshot` before/after diffing was considered and **rejected**:
(a) the CLI isn't in the server image (extra moving part); (b) snapshot is lossy
by design — it strips secrets and server-set fields, masks values, and rewrites
ids to client-refs (`#domain-b`) — so it cannot assert exact field values; (c)
it is built for backup/migration, not per-field verification. The direct-client
read gives a *more* precise "exactly these fields are as expected" guarantee
without the CLI dependency. Revisit only if a future need (e.g. asserting the
absence of unexpected *objects*, not fields) calls for a whole-state diff.

## Test/CI ENVIRONMENT CONSTRAINTS (Claude Code on the web)

These are specific to the sandboxed remote execution environment; a developer
laptop or CI runner will differ.

- **Docker is installed but the daemon is NOT running and there is no socket.**
  `docker` and `dockerd` binaries exist at `/usr/bin`. Start it manually:
  `sudo -n dockerd >/tmp/dockerd.log 2>&1 &` then wait ~8s. It comes up as
  v29.3.1 with the `overlayfs` storage driver. cgroup v1, "No cpuset support"
  warnings are harmless.
- **`dockerd` does NOT persist across conversation turns** — re-check with
  `pgrep -x dockerd` and restart if needed at the start of any turn that uses it.
- **Container image pulls are BLOCKED by the network allowlist.** The registry
  manifest endpoints are reachable (`registry-1.docker.io` → 404,
  `ghcr.io` → 301 are normal responses), but the **blob/layer CDNs return 403**:
  - `production.cloudfront.docker.com` (Docker Hub layer blobs)
  - `pkg-containers.githubusercontent.com` (GHCR layer blobs)
  These two hosts must be added to the environment's network allowlist before
  `docker pull stalwartlabs/stalwart:v0.16` (or the `ghcr.io/...` mirror) can
  succeed. A 403 from `curl https://<host>/` is the signature of an allowlisted-
  denied host (reachable hosts give 301/404/200). Docker Hub anonymous pulls can
  also hit a separate rate limit ("unauthenticated pull rate limit").
- Go module proxy (`proxy.golang.org`) and `github.com` are reachable.

### DECISION: iterate acceptance tests via CI, not locally (2026-06-01)

Running the acceptance harness *locally in this Claude Code web environment* is
**deferred**. The blocker is pulling the Stalwart image:

- The `*.githubusercontent.com` wildcard was added to the allowlist and works for
  sibling subdomains (`objects`/`avatars`/`camo` all pass), but
  `pkg-containers.githubusercontent.com` (GHCR blob host) remains specifically
  denied — the proxy returns `403` with header `x-deny-reason: host_not_allowed`,
  i.e. a more specific rule shadows the wildcard. An explicit entry was requested
  but had not taken effect within the session.
- Docker Hub's CDN (`production.cloudfront.docker.com`) was unblocked, but
  anonymous pulls from the shared egress IP hit Docker Hub's
  unauthenticated **rate limit** (even `hello-world` fails). Would need a
  `docker login` with a Docker Hub token to get past it.

Decision: rather than keep fighting the environment, **rely on GitHub Actions**
for acceptance-test execution. GitHub-hosted runners ship Docker and pull public
images without these restrictions. The workflow runs the `testacc` job on every
PR/push; iterate by reading the Actions job logs and pushing fixes until green.
Revisit local execution only if/when the registry block is lifted (re-check with
`curl -D - https://pkg-containers.githubusercontent.com/` — absence of the
`x-deny-reason` header means it's reachable, then `docker pull` should work).

## Tooling versions / gotchas

- `.golangci.yml` is **golangci-lint v2 config format** (`version: "2"`). This
  requires **golangci-lint v2.x** and **golangci-lint-action@v7+**; the older
  `@v6` action installs golangci-lint v1.x and fails to parse a v2 config with
  **exit code 3** (config load error, distinct from exit 1 = lint findings).
  CI pins `version: v2.5.0`. Local installed version is also v2.5.0.
- Go toolchain in the environment auto-upgrades (`go.mod` shows `go 1.25.8`).
- The provider's `errcheck` linter requires explicitly discarding deferred
  `Close()` errors: `defer func() { _ = x.Close() }()`.

## Verification commands

```sh
go build ./... && go vet ./... && go test ./...   # unit tests, no network
gofmt -l -s .                                      # must print nothing
golangci-lint run ./...                            # needs v2.x
make testacc                                        # acceptance (needs container)
```
