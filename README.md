# Terraform Provider for Stalwart

A [Terraform](https://www.terraform.io) provider for the
[Stalwart](https://stalw.art) mail and collaboration server, built on the modern
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The provider targets **Stalwart v0.16 and later**, which removed the legacy REST
management API and now exposes all configuration as
[JMAP](https://jmap.io) objects through the management API at the `/api`
endpoint (negotiated via the `urn:stalwart:jmap` capability). See the
[Stalwart schema reference](https://stalw.art/docs/ref/) for the underlying
object definitions.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- A Stalwart server running v0.16 or later
- [Go](https://go.dev/dl/) >= 1.24 (to build the provider)

## Using the provider

```hcl
terraform {
  required_providers {
    stalwart = {
      source = "flungo/stalwart"
    }
  }
}

provider "stalwart" {
  endpoint = "https://mail.example.com" # base URL; the client appends /api
  token    = var.stalwart_token         # or set STALWART_TOKEN
}
```

### Authentication

The provider authenticates with **either** a bearer token **or** a
username/password pair:

| Setting    | Environment variable  | Notes                                          |
| ---------- | --------------------- | ---------------------------------------------- |
| `endpoint` | `STALWART_ENDPOINT`   | Base URL of the server. The provider appends `/api`. |
| `token`    | `STALWART_TOKEN`      | Bearer token. Takes precedence over username/password. |
| `username` | `STALWART_USERNAME`   | HTTP Basic auth username.                       |
| `password` | `STALWART_PASSWORD`   | HTTP Basic auth password.                       |

Explicit configuration takes precedence over environment variables. Provide a
`token`, or a `username`/`password` pair — not both.

## Resources and data sources

### Resources

| Resource                    | Stalwart object | Notes |
| --------------------------- | --------------- | ----- |
| `stalwart_domain`           | `Domain`        | Email domain, DKIM/DNS/TLS management modes, catch-all, aliases. |
| `stalwart_dkim_signature`   | `DkimSignature` | DKIM signing key (`ed25519-sha256` or `rsa-sha256`). |
| `stalwart_account`          | `Account` (`@type: User`)  | Individual account: quota, roles, group membership. |
| `stalwart_group`            | `Account` (`@type: Group`) | Group account. Membership is set from the account side. |
| `stalwart_mailing_list`     | `MailingList`   | Mailing list with recipient addresses. |
| `stalwart_role`             | `Role`          | A named set of permissions. |

### Data sources

| Data source                 | Description |
| --------------------------- | ----------- |
| `data.stalwart_domain`      | Reads a domain by name. |
| `data.stalwart_account`     | Reads an account (user or group) by email address. |
| `data.stalwart_dns_records` | Reads the DNS record recommendations (`dnsZoneFile`) for a domain. |

### Referencing a domain

Child objects (accounts, groups, mailing lists, DKIM signatures) belong to a
domain. Reference it either by the domain's opaque id (the idiomatic Terraform
approach, which also tracks the dependency), or by name:

```hcl
resource "stalwart_account" "alice" {
  domain_id = stalwart_domain.example.id # preferred
  # domain  = "example.com"              # alternative: resolved by name lookup
  name      = "alice"
}
```

Exactly one of `domain_id` or `domain` must be set.

### Importing

| Resource                  | Import ID            | Example |
| ------------------------- | -------------------- | ------- |
| `stalwart_domain`         | domain name          | `terraform import stalwart_domain.example example.com` |
| `stalwart_account`        | email address        | `terraform import stalwart_account.alice alice@example.com` |
| `stalwart_group`          | email address        | `terraform import stalwart_group.team team@example.com` |
| `stalwart_mailing_list`   | email address        | `terraform import stalwart_mailing_list.announce announce@example.com` |
| `stalwart_role`           | description          | `terraform import stalwart_role.support "Support team role"` |
| `stalwart_dkim_signature` | opaque id (ULID)     | `terraform import stalwart_dkim_signature.example 01ARZ3NDEKTSV4RRFFQ69G5FAV` |

Each of these also accepts the object's opaque id (ULID) directly.

A complete example lives in [`examples/`](./examples).

## Developing the provider

Build and run the unit tests:

```sh
make build
make test
```

Install the provider into the local plugin mirror so it can be used from a local
Terraform configuration (see the [CLI dev override docs](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)):

```sh
make install
```

Format, vet, and lint:

```sh
make fmt
make lint
```

Generate the registry documentation (uses
[`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs)):

```sh
make generate
```

### Acceptance tests

Acceptance tests create and destroy **real** resources and therefore require a
live Stalwart test instance. They are guarded by `TF_ACC` and read the same
credentials as the provider:

```sh
export STALWART_ENDPOINT=https://mail.test.example.com
export STALWART_TOKEN=...
make testacc
```

> **Warning:** acceptance tests will create and destroy objects on the target
> server. Always point them at a dedicated, disposable test instance.

## Releasing

Releases are produced by [GoReleaser](https://goreleaser.com) and published to
the Terraform Registry by the [`release`](./.github/workflows/release.yml)
GitHub Actions workflow when a `v*` tag is pushed. The workflow GPG-signs the
checksums; configure the `GPG_PRIVATE_KEY` and `PASSPHRASE` repository secrets
with the key registered with the Terraform Registry.

## License

[Mozilla Public License v2.0](./LICENSE).
