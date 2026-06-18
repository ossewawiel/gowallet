---
description: Design a gowallet vertical slice and open a GitHub issue that fully specs it for a build session.
argument-hint: <slice id or short description, e.g. "S2 earn + balance">
---

We're **designing a vertical slice** for gowallet. This is design-only — **no production code**.

**Slice:** $ARGUMENTS

## Do this in order

1. **Pull the house rules** (skills): `go-architecture`, `rest-api-standards`, `tdd-workflow`.
2. **Read the knowledge** (only what's needed): `docs/specifications.pdf` (always — it's the final
   word), `docs/SLICES.md`, `docs/ACCEPTANCE.md`, `docs/ARCHITECTURE.md`, `docs/REST_API_GUIDELINES.md`.
3. **Design the slice** — produce, concretely:
   - The **REST cycle** it delivers (endpoints + methods).
   - The **OpenAPI fragment** (new paths/schemas to add to `api/openapi.yaml`).
   - **Data-model deltas** — the goose migration up/down (tables, indexes, `UNIQUE(ref)` etc.).
   - **Domain rules** and the sentinel errors involved.
   - **Invariants** — assign `INV-n` IDs; these become rows in `docs/ACCEPTANCE.md` and the issue.
   - The **red-test list** — the exact failing tests to write first (unit + acceptance).
   - **Acceptance criteria**, **depends-on**, and **stream** (A/B/C from SLICES.md).
4. **Open or enrich the issue** — first check whether a slice issue already exists
   (`gh issue list --label slice --search "<id>:"`):
   - **If it exists** (S0–S5 were pre-created at Step-3 kickoff): **enrich it, don't duplicate** —
     `gh issue edit <n> --body-file <tmp>` to add the full design (OpenAPI fragment, migration,
     red-test list) on top of the existing kickoff prompt.
   - **If not:** create it — `gh issue create --title "<id>: <name>" --label slice --body-file <tmp>`
     using the fields from `.github/ISSUE_TEMPLATE/slice.yml`.
   Either way the issue must be complete enough that a fresh session builds it with **zero further
   design questions**.
5. **Register invariants** — dispatch the `doc-updater` subagent to add the new `INV-n` rows to
   `docs/ACCEPTANCE.md` (status ⬜) and log a PROMPT_LOG entry.

## Report back (house voice: visual, low lingo)
A frame with: the slice summary, the new `INV-n` IDs, dependencies/stream, and the **issue URL**.
Then tell me the next step (usually: `/build-slice <issue#>`).
