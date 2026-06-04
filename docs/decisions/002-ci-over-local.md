# 002 — Run acceptance tests via CI, not locally in the Claude Code web environment

**Status**: Accepted (2026-06-01)

## Context

The acceptance test harness requires pulling the `stalwartlabs/stalwart:v0.16` Docker image. In the Claude Code on the web environment, two separate network restrictions block this:

1. **GHCR blob CDN blocked**: `pkg-containers.githubusercontent.com` returns `403` with `x-deny-reason: host_not_allowed`. A `*.githubusercontent.com` wildcard allowlist entry was added but a more specific deny rule shadows it — only an explicit entry for this exact host would fix it.

2. **Docker Hub rate limit**: `production.cloudfront.docker.com` (Docker Hub's blob CDN) was unblocked, but anonymous pulls from the shared egress IP hit Docker Hub's unauthenticated pull rate limit. Even `hello-world` fails. A `docker login` with a personal Docker Hub token would be needed.

Additionally, `dockerd` does not persist across conversation turns and must be restarted manually each session.

## Decision

**Rely on GitHub Actions for acceptance-test execution.** GitHub-hosted runners ship Docker and can pull public images without these restrictions. The `testacc` job runs on every PR and push via `.github/workflows/test.yml`. The workflow to iterate is: push a fix, read the Actions job log, push another fix.

## How to check if the restriction has been lifted

```sh
curl -D - https://pkg-containers.githubusercontent.com/
```

If the response contains `x-deny-reason: host_not_allowed`, the block is still in place. If the response is a normal 301/404/200 (no `x-deny-reason`), the host is reachable and `docker pull` should work.

If both CDN restrictions are resolved, local acceptance tests can be run with:

```sh
sudo -n dockerd >/tmp/dockerd.log 2>&1 &
sleep 8
make testacc
```

## Consequences

- Acceptance test feedback is slower (requires a CI round-trip instead of a local run).
- New resources must be validated by pushing to the feature branch and checking the CI job log.
- The constraint is specific to the Claude Code web environment — developer laptops and CI runners are unaffected.
