---
name: go-architecture
description: gowallet's Go architecture and code conventions — clean layering (api→domain→store), project layout, the wire-crossing safety rule, SQLite/sqlc/goose patterns, and error design. Use when writing or reviewing Go code, structuring packages, designing the data layer, or making any change to handlers/domain/store.
---

# Go architecture & code conventions

You are working on **gowallet**. Before writing code, anchor on `docs/ARCHITECTURE.md` (read it if
you haven't this session). Enforce these without exception:

## Layering (one direction only)
- `api → domain → store`. **`domain` imports nothing upward** — it's pure Go, unit-testable with no DB.
- `domain` defines repository **interfaces**; `store` implements them. `api` never imports `store`.
- Generated code lives in `internal/*/gen/` and is **never hand-edited** (oapi-codegen, sqlc output).

## The wire-crossing rule (multi-user safety)
- The ONLY things shared across requests are the **`*sql.DB` pool** and **config**.
- Everything request-specific (account_id, role, deadline) rides in **`r.Context()`**.
- Handlers read identity from **context**, never from the URL or a shared field.
- No package-level mutable state. No per-request data stored on a shared struct.

## Database patterns
- Open SQLite with PRAGMAs: `journal_mode=WAL`, `busy_timeout=5000`, `synchronous=NORMAL`,
  `foreign_keys=ON`. Single writer (`SetMaxOpenConns(1)` on the write path).
- Money/points are **integers**. Never floats.
- Invariants are **constraints**: `UNIQUE(ref)` for idempotency; balance check + write in the **same
  transaction**; never allow a negative balance.

## Code style
- Idiomatic Go: small functions, accept interfaces / return structs, wrap errors with `%w`.
- Sentinel domain errors (`ErrNotFound`, `ErrInsufficientBalance`, `ErrDuplicateRef`, `ErrForbidden`,
  `ErrInvalidInput`); map them to HTTP status in **one place** in the api layer.
- `gofmt` + `go vet` + `golangci-lint` clean, always. Run `-race` on tests.

## When unsure
The brief (`docs/specifications.pdf`) wins. If a choice conflicts with the locked stack in
`CLAUDE.md`, flag it — don't silently diverge.
