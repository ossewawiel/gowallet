---
name: tdd-workflow
description: gowallet's strict TDD and vertical-slice workflow — spec-first red/green/refactor, the two-layer testing source of truth (OpenAPI+Schemathesis for contract, Go -race tests for invariants/concurrency), the quality gate, and the per-slice acceptance registry. Use when building a slice, writing tests, running the quality gate, or deciding what "done" means.
---

# TDD & vertical-slice workflow

Building a **gowallet** slice. Anchor on `docs/DEVELOPMENT_FLOW.md` and `docs/ACCEPTANCE.md`.
Never write production code before a failing test. Hold this line.

## The loop (per slice)
1. **DESIGN** → the GitHub issue is the spec (from `/design-slice`). Re-read it fully.
2. **RED** 🔴 — spec-first:
   - Extend `api/openapi.yaml`; regenerate (`oapi-codegen`).
   - Write failing tests BELOW it: domain unit tests + one acceptance test per invariant the issue
     lists. Run → confirm they fail for the **right reason**.
3. **GREEN** 🟢 — minimum code to pass (migration → sqlc query → domain rule → handler). No gold-plating.
4. **REFACTOR** 🧹 — clean names/dedupe; tests stay green.
5. **PROVE** 🧪 — the quality gate (below).
6. **LOG + SHIP** 📦 — PROMPT_LOG entry (via `doc-updater`), flip ACCEPTANCE rows to ✅, push, PR closes issue.

## Two-layer testing source of truth
- **Layer 1 — contract:** `api/openapi.yaml` fuzzed by **Schemathesis** (property + stateful). Catches
  shape/status/sequence bugs.
- **Layer 2 — invariants:** Go `-race` tests in `test/acceptance/` proving `docs/ACCEPTANCE.md` rows.
  Catches business rules + concurrency + wire-crossing (the parts Schemathesis can't see).
- Concurrency invariants = **parallel-submission tests** under `-race` (same-ref race → once;
  concurrent spends → no overdraw; N users → no cross-leak).

## Quality gate (definition of done — all green)
`gofmt` ✓ · `go vet` ✓ · `golangci-lint` ✓ · `go build ./...` ✓ · `go test -race ./...` ✓ ·
`schemathesis run` ✓ · the slice's `ACCEPTANCE.md` rows ✓ · `PROMPT_LOG.md` updated ✓.

## Acceptance registry discipline
- Each new invariant → a row in `docs/ACCEPTANCE.md` (next `INV-n`, named test, slice, status ⬜).
- ⬜ planned → 🟡 test written/red → ✅ green. Final testing = the whole table green at once.

## Report back visually
Every run ends with a status frame/table: what ran, ✅/🔴 per check, and the smallest next step on failure.
