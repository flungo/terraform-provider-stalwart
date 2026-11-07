# Design Decisions

Lightweight ADR-style records for non-obvious choices made in this project.

| # | Decision | Status |
| --- | --- | --- |
| [001](001-test-methodology.md) | Use direct JMAP client for server-side assertions in acceptance tests | Accepted |
| [002](002-ci-over-local.md) | Run acceptance tests via CI, not locally in the Claude Code web environment | Accepted (2026-06-01) |
| [003](003-ordered-map-collections.md) | Model order-significant `Map<T>` fields as `types.List` | Accepted (2026-08-02) |
| [004](004-dual-server-version-support.md) | Support Stalwart v0.16 and v1 from one provider release | Accepted (2026-11-07) |
