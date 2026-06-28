# Releasing terraform-provider-stalwart

This document covers the end-to-end release process: versioning decisions,
how to cut a release, and how to verify it landed correctly.

## One-time setup (already done)

### GPG signing key

Both registries require every provider release to include a GPG signature over
the `SHA256SUMS` file. The release workflow reads two repository secrets:

| Secret | How to obtain |
|---|---|
| `GPG_PRIVATE_KEY` | `gpg --armor --export-secret-keys <fingerprint>` |
| `PASSPHRASE` | The passphrase for the key |

The corresponding **public key** must be registered at
[registry.terraform.io](https://registry.terraform.io) under your account →
**GPG Keys**. Without it the Terraform Registry accepts the release files but
cannot verify the signature and will reject ingestion.

The same GPG public key is stored in the OpenTofu registry's provider metadata
at `providers/f/flungo/stalwart.json` in
[opentofu/registry](https://github.com/opentofu/registry). This was submitted
once as part of the initial provider registration PR and does not need to change
unless the signing key is rotated (see [Key rotation](#key-rotation) below).

### Terraform Registry connection

The Terraform Registry is connected to this repo via a **GitHub App** (visible
under `github.com/flungo/terraform-provider-stalwart` → Settings → GitHub
Apps). This app fires on every new GitHub release and triggers automatic
ingestion — no manual sync needed after the first connection was established.

### OpenTofu Registry registration

The provider is registered at
[search.opentofu.org/provider/flungo/stalwart](https://search.opentofu.org/provider/flungo/stalwart)
via a one-time PR to [opentofu/registry](https://github.com/opentofu/registry)
(PR [#4489](https://github.com/opentofu/registry/pull/4489), now merged). No
further one-time setup is required.

---

## Versioning scheme

| Phase | Version range | When to use |
|---|---|---|
| Active v0 development | `v0.x.y` | While the Stalwart instance migration to Terraform is in progress |
| First stable release | `v1.0.0` | Once the migration is complete and the provider API is considered stable |

Within v0, follow semver loosely:
- **Minor bump** (`v0.x.0`): new resources or data sources; any breaking schema change
- **Patch bump** (`v0.x.y`): bug fixes, documentation, non-breaking improvements
- **Pre-releases** (`v0.x.y-alpha.n`, `-beta.n`, `-rc.n`): use for early testing;
  GoReleaser marks them as GitHub pre-releases automatically

---

## Cutting a release

### Option A — workflow_dispatch (recommended)

Use the Actions UI or trigger via the GitHub API. This creates the tag automatically
if it doesn't already exist.

1. Go to **Actions → release → Run workflow**
2. Enter the version (e.g. `v0.2.0`). Must start with `v`.
3. Click **Run workflow**.

Or via Claude Code / MCP:
```
mcp__github__actions_run_trigger
  method: run_workflow
  workflow_id: release.yml
  ref: main
  inputs: { version: "v0.2.0" }
```

### Option B — push a tag manually

```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

Both options are equivalent. The workflow checks out the tag, runs GoReleaser,
and publishes the GitHub release with signed binaries.

---

## What the release workflow does

1. Creates and pushes the tag (workflow_dispatch only; skipped if tag exists)
2. Imports the GPG key from repository secrets
3. Runs GoReleaser, which:
   - Builds binaries for all supported platforms (linux/darwin/windows/freebsd × amd64/arm64/386/arm)
   - Creates `.zip` archives
   - Generates `SHA256SUMS`
   - Signs `SHA256SUMS` with the GPG key → `SHA256SUMS.sig`
   - Creates a GitHub release and uploads all artifacts
4. The Terraform Registry GitHub App detects the new release and ingests it
5. The OpenTofu Registry automatically picks up the new release within ~15 minutes
   (a scheduled workflow in opentofu/registry polls GitHub releases on that cadence)

---

## Verifying a release

1. **GitHub release**: check `github.com/flungo/terraform-provider-stalwart/releases` —
   should show the new version with all platform zips, `SHA256SUMS`, and `SHA256SUMS.sig`.
2. **Terraform Registry ingestion**: check `registry.terraform.io/providers/flungo/stalwart` —
   the new version should appear within a few minutes. If it doesn't:
   - Verify the `.sig` file is present in the GitHub release assets
   - Go to the Registry provider page and use **Resync** to manually trigger ingestion
3. **OpenTofu Registry ingestion**: check
   `search.opentofu.org/provider/flungo/stalwart` — the new version should
   appear within ~15 minutes (the registry polls on a 15-minute cron). No manual
   action is required; if it hasn't appeared after 30 minutes, verify the
   `SHA256SUMS.sig` file is present in the GitHub release assets.
4. **Regression test**: the release workflow does not run the regression test. After a
   stable release, optionally trigger `provider-regression.yml` in `flungo/stalwart.flungo.net`
   via `workflow_dispatch` to confirm the config still applies cleanly against the released binary.

---

## Updating the config repo after a release

After publishing a stable release, update the version constraint in
`flungo/stalwart.flungo.net`:

```hcl
# terraform/versions.tf
required_providers {
  stalwart = {
    source  = "flungo/stalwart"
    version = "= 0.2.0"   # bump to the new version
  }
}
```

Commit the change on a branch, let the `fresh` CI job verify it downloads and applies
cleanly, then merge to main.

---

## Troubleshooting

### "Missing SHASUMS signature file" on the Registry

The `GPG_PRIVATE_KEY` or `PASSPHRASE` secret is absent, empty, or incorrect, OR
the public key is not registered on the Registry. Check:
1. Both secrets are set in the provider repo (Settings → Secrets → Actions)
2. The public key fingerprint matches what's registered at registry.terraform.io

### Release not appearing on the Registry

- Confirm the GitHub App is still installed (Settings → GitHub Apps → Terraform Registry)
- Check the release assets include `SHA256SUMS.sig`
- Use **Resync** on the Registry provider page to manually trigger ingestion

### Tag already exists when using workflow_dispatch

The workflow skips tag creation and builds from the existing tag. This is safe — use
it to re-run a release if the workflow failed partway through.

### GoReleaser releases to the wrong tag (422 asset-already-exists errors)

This happens when a pre-release tag (e.g. `v0.1.0-alpha.3`) and the new stable
tag (e.g. `v0.1.0`) both point to the same commit. GoReleaser uses `git describe`
to detect the current version and can pick the pre-release tag instead of the
intended one, then tries to upload assets to the existing pre-release GitHub
release — which already has them — and fails with HTTP 422.

The release workflow passes `GORELEASER_CURRENT_TAG` to prevent this, but if you
encounter it (e.g. after pushing a tag manually on a commit that already has a
pre-release tag):
1. Delete the incorrectly-targeted GitHub release (do **not** delete the tag itself)
2. Re-run the release workflow — `GORELEASER_CURRENT_TAG` will ensure GoReleaser
   creates a fresh release at the correct tag

To avoid it entirely, cut stable releases from a fresh commit rather than tagging
a commit that already carries a pre-release tag.

### Pre-release versions not visible as "latest" on the Registry

GoReleaser marks versions with a pre-release suffix (e.g. `-alpha.1`) as GitHub
pre-releases. The Registry reflects this — pre-release versions are listed but
not shown as the current stable version.

### New version not appearing on the OpenTofu Registry after 30 minutes

The OpenTofu Registry polls GitHub releases every 15 minutes. If a version is
still absent after 30 minutes:

1. Confirm the GitHub release is not a draft and the `SHA256SUMS.sig` file is
   present in its assets.
2. Check the `bump-versions` workflow runs at
   `github.com/opentofu/registry/actions` — look for failures around the time
   of the release.
3. If the workflow failed, open an issue at
   `github.com/opentofu/registry/issues` referencing the provider and version.

---

## Key rotation

If the GPG signing key needs to be replaced (e.g. it is compromised or expires):

1. **Generate a new key pair** and export the private key.
2. **Update repository secrets**: replace `GPG_PRIVATE_KEY` and `PASSPHRASE` in
   Settings → Secrets → Actions.
3. **Update the Terraform Registry**: go to `registry.terraform.io` → your
   account → GPG Keys and add the new public key. The old key can remain so
   that previously-signed releases continue to verify.
4. **OpenTofu Registry**: no action required. This provider was registered via
   repository URL only (no GPG public key was submitted to opentofu/registry),
   so the OpenTofu Registry has no stored key to rotate. New releases signed
   with the new key will be ingested as normal.
5. Cut the next release — both registries will use the new key from that point
   on.
