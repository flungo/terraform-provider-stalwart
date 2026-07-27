# CLAUDE.md

Operational rules and quick-reference for AI coding sessions on this repository.
Detailed reference lives in [`docs/`](docs/README.md) — read it when implementing or extending resources.

## Project shape

- Terraform provider on the **Plugin Framework** (not the legacy SDK).
- Targets **Stalwart v0.16+**, which exposes all configuration as JMAP objects (no REST management API).
- `internal/client/` — minimal JMAP client. `internal/provider/` — resources and data sources.
- See [docs/architecture.md](docs/architecture.md) for package layout and design patterns.

## Branch management

Claude sessions must **never commit directly to `main`**.
All work happens on a feature branch.

**At the start of every session:**

- If `main` is checked out: pull to ensure it is up to date, then create a new feature branch before making any changes.
- If a non-`main` branch is already checked out: confirm with the user whether to continue on that branch or start fresh before proceeding.

**After each user prompt:**

Fetch from upstream and, if there are new commits on `main`, rebase the current feature branch onto it:

```sh
git fetch origin main
git rebase origin/main   # only if fetch produced new commits
```

Before continuing with the next task, review what changed on `main`.
Read the diff and any updated docs or decision records to understand what new facts or decisions were introduced.
Then:

- If the upstream changes affect work already done on the branch, apply the necessary adjustments (via rebase amend — see below).
- If anything is unclear, contradicts the goal of the current session, or conflicts with a decision already made on the branch, **stop and ask the user to confirm the direction** before proceeding. Do not silently resolve ambiguous conflicts by picking one interpretation.

**Commit message convention — Conventional Commits:**

All commits must use [Conventional Commits](https://www.conventionalcommits.org/) prefixes: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, etc. The subject line is imperative mood, no trailing period.
Keep the body for the *why*, not a re-statement of the diff.

**Landing branches — always via PR:**

Claude never pushes directly to `main`.
When a branch is ready to land, open a PR and let the user merge it.
This keeps `main` protected and provides a review gate even for small changes.
After a PR is merged, delete the remote branch.

**Force-push policy:**

Force-pushing is allowed on feature branches (necessary after `--amend` or interactive rebase).
Never force-push `main`.

**Linear history — no merge commits:**

This repo maintains a strictly linear history.
Never create merge commits.
All branches land on `main` via either squash or rebase, never `git merge`.

**Squash vs rebase when merging to main:**

- **Squash** when the branch is a single logical change, regardless of how many working commits it took to get there (e.g. one resource added, one doc fix, one ADR written). The squashed commit message should describe the change, not the journey.
- **Rebase** (fast-forward, no squash) when the branch contains multiple distinct logical changes that are worth preserving individually in the history (e.g. separate commits for a new resource, schema tests, and a runbook update).

When in doubt, squash — a clean single commit is easier to revert and easier to read in `git log`.

**Rebase hygiene — no "fix-up" commits on a branch:**

When work on a branch contains a minor inaccuracy (typo, wrong value, incorrect claim), amend or fixup the relevant existing commit rather than appending a new corrective commit.
The branch history should read as though those changes were always correct — not as a record of corrections made after the fact.
This applies both when correcting work in response to upstream changes on `main` and when self-correcting during the current session.

## Stalwart API — critical facts

Full reference: [docs/stalwart-api.md](docs/stalwart-api.md).
The most common traps:

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
git clone --depth 1 https://github.com/stalwartlabs/website.git    /tmp/stalwart-website
# Object schemas:
ls /tmp/stalwart-website/src/content/docs/docs/ref/object/
```

Key paths in `stalwart-src`:

- Object schemas: `crates/registry/src/schema/structs.rs`
- Collection wire formats: `crates/registry/src/types/{map,list}.rs`
- HTTP routing: `crates/http/src/request.rs`
- Method/capability naming: `crates/jmap-proto/src/request/{capability,method}.rs`

## Acceptance tests — CI only (web environment)

**Do not attempt to run acceptance tests locally in the Claude Code web environment.** Container image pulls are blocked by network restrictions.
Push to the feature branch and iterate against the CI `testacc` job in GitHub Actions.

See [docs/decisions/002-ci-over-local.md](docs/decisions/002-ci-over-local.md) for the full context and instructions for checking whether the restriction has been lifted.

## Registry documentation

Terraform Registry docs live in `docs/` (generated by `tfplugindocs` from schema descriptions and `examples/`).

**After adding or modifying a resource or data source:**

1. Add or update the example file in `examples/resources/<name>/resource.tf` (or `data-sources/<name>/data-source.tf`).
2. Add or update `examples/resources/<name>/import.sh` for resources that support import.
3. **Regenerate docs.** In the web environment you cannot install Terraform locally, so push the branch — the `docs` CI workflow installs Terraform, regenerates the docs, and commits the generated output back to the branch automatically. If Terraform is available locally, run `make generate` before pushing.

**Documentation structure:**

- `templates/index.md.tmpl` — provider overview page (auth table, narrative)
- `examples/resources/<resource>/resource.tf` — rendered as the resource example on the Registry
- `examples/resources/<resource>/import.sh` — rendered as the Import section
- `docs/` — auto-generated output; committed to the repo so the Registry can read it

**Do not hand-edit files under `docs/resources/`, `docs/data-sources/`, or `docs/index.md`** — they are regenerated on every run of `make generate` and changes will be lost.

## Markdown validation

Markdown is style-linted and link-checked in CI by the shared flungo/github-workflows callers (`markdown-lint.yml` and `markdown-links.yml`, both `@v1`).
markdownlint rules live in `.markdownlint-cli2.jsonc`; URLs that legitimately 403/404 while unauthenticated go in `.lycheeignore`.
The generated docs (`docs/index.md`, `docs/resources/`, `docs/data-sources/`) are excluded from markdownlint — tfplugindocs owns their formatting, so the two never pull in different directions — while lychee still link-checks them.

To reproduce CI locally, match the pinned tool versions:

- markdownlint-cli2 **0.17.2** (markdownlint 0.37.4) — the version bundled by
  `DavidAnson/markdownlint-cli2-action@v19`. Install it with
  `npm install markdownlint-cli2@0.17.2`, then run `markdownlint-cli2 '**/*.md'`.
- lychee for the offline link and anchor check (the action bundles its own):
  `cargo install lychee --locked`, then
  `lychee --offline --include-fragments --no-progress '**/*.md'`.

The prose and cross-reference conventions these rules pair with are not repeated here.
Semantic line breaks (paired with `MD013` off) and the wider doc-tree standards come from the `docs-standards@flungo-plugins` plugin ([flungo/claude-plugins](https://github.com/flungo/claude-plugins)), enabled for this repo in [`.claude/settings.json`](.claude/settings.json).
The link/anchor and markdownlint-rule pairings — cross-reference hygiene, `MD024` unique headings, `MD028` adjacent blockquotes — are documented in [flungo/github-workflows markdown-validation.md](https://github.com/flungo/github-workflows/blob/main/docs/reference/markdown-validation.md).

## Verification commands

```sh
go build ./... && go vet ./... && go test ./...   # unit tests, no network
gofmt -l -s .                                      # must print nothing
golangci-lint run ./...                            # needs v2.x
make testacc                                       # acceptance (needs container — CI only in web env)
make generate                                      # regenerate Registry docs (needs Terraform in PATH)
```

## Tooling gotchas

- `.golangci.yml` uses **golangci-lint v2 config format** — requires v2.x. The `@v6` GitHub Action installs v1.x and fails with exit code 3.
- The `errcheck` linter requires explicitly discarding deferred `Close()` errors: `defer func() { _ = x.Close() }()`.
- Go toolchain auto-upgrades in this environment (`go.mod` shows `go 1.25.8`).
