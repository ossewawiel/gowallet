# 🧩 SOLUTION.md — Design, Trade-offs & AI Workflow

This document explains **why gowallet is built the way it is**, in plain language, and shows
**how AI was used** during the build (a requirement of the brief). It grows alongside the code.

---

## 1. Approach in one paragraph

gowallet is a loyalty **points wallet**: accounts earn and spend points, and the service must
keep balances correct even when the same transaction is submitted twice or many requests land at
once. The guiding principle is **boring correctness over clever code** — a clear data model, a
single source of truth for balances, idempotent writes keyed on the transaction `ref`, and a
database that survives restarts. It's deliberately built in stages (plumbing → dev system →
design & build) so the reasoning is visible, not just the final artifact.

---

## 2. Key decisions so far

Each decision below was made after weighing alternatives. The full reasoning and sources are in
[`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md).

| # | Decision | Alternatives considered | Why we chose it |
|---|----------|-------------------------|-----------------|
| D1 | **SQLite** as the relational DB | PostgreSQL, MariaDB | Brief recommends it; zero external services; perfect for a locally-runnable take-home. Data layer kept swappable for a future Postgres move. |
| D2 | **`modernc.org/sqlite`** (pure-Go driver) | `mattn/go-sqlite3` (CGO) | No C compiler needed → reviewers can `git clone && go run` on any machine. Accepts a write-speed trade-off that's irrelevant at this scale. |
| D3 | **Persist to a single `.db` file** | In-memory, server DB | Meets "durable across restarts" with the least moving parts. |
| D4 | **Public GitHub repo** | Private + invite | Simplest to share/review for the assignment. |
| D5 | **OpenAPI 3 + Swagger UI** | Hand-written docs, Postman collection | Living, testable API docs; reviewers can click-to-call endpoints. |
| D6 | **Strict TDD** + Playwright for end-to-end | Test-after | Correctness is the whole point of a wallet; tests are the spec. |
| D7 | **Docker** for portable runs | Local-only | "Runs anywhere" without requiring a Go install on the reviewer's box. |
| D8 | **stdlib `net/http` (1.22) + `chi`** | gin / echo / fiber | Stdlib now does method routing; chi adds middleware + sub-routers without framework lock-in. Idiomatic, minimal deps. |
| D9 | **Spec-first `oapi-codegen`** (strict-server) + `kin-openapi` | code-first (Huma/swaggo), hand-rolled | `api/openapi.yaml` is the single source of truth *and* the TDD target; codegen keeps code and spec in lockstep. |
| D10 | **`sqlc`** + **`goose`** (timestamped) | GORM, sqlx | Type-safe SQL, zero runtime reflection; timestamped migrations survive parallel branches. |
| D11 | **JWT HS256** (`golang-jwt`, method-pinned) | opaque tokens, PASETO, RS256 | Single service signs + verifies → symmetric is correct; `WithValidMethods` kills alg-confusion. `role`+`sub` claims for member/admin. |
| D12 | **Schemathesis** (contract) + Go **`-race`** (invariants) | Playwright on contract, Dredd | Two-layer source of truth: spec-fuzz for shapes, race tests for business rules + concurrency. |
| D13 | **Issue-driven vertical slices**, 3 streams, auth midstream | one-shot build | Each GitHub issue fully specs a slice so a fresh agent session builds with no re-design. |

> ✅ **Resolved in Step 2** (these were "open"): router = stdlib + chi, auth = JWT HS256, concurrency
> = WAL + `busy_timeout` + single-writer + `UNIQUE(ref)`. **Step 3 is execution** — building the
> slices in `docs/SLICES.md` via the issue-driven TDD flow (`docs/DEVELOPMENT_FLOW.md`).

---

## 3. Correctness model (the part that matters)

The brief's hard constraints and how the design will satisfy them:

| Constraint | Strategy (to be implemented in Step 3) |
|------------|----------------------------------------|
| No double-counting the same `ref` | `UNIQUE` constraint on transaction `ref`; inserts are idempotent. |
| No spend below zero | Balance check **inside** the same transaction that writes the spend. |
| Safe under overlapping requests | SQLite **WAL** + `busy_timeout`; serialise writes; atomic transactions. |
| Durable across restarts | On-disk SQLite file; `synchronous=NORMAL` with WAL. |

---

## 4. 🤖 AI workflow — what was asked, accepted, edited, and why

The brief asks for a short, honest account of how AI was used. The **full timeline** is in
[`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md); this is the summary.

- **How I used it:** as an *interrogator and pair-engineer*, not an autocomplete. Before any
  setup, the assistant pushed back with hard questions (driver choice, repo visibility, install
  method, testing posture) and cited primary sources (go.dev, sqlite.org) so decisions were made
  on evidence, not vibes.
- **What I accepted:** the pure-Go SQLite driver recommendation (portability win), the
  WAL + `busy_timeout` concurrency guidance, and the staged plumbing → dev → design plan.
- **What I steered/edited:** chose a **public** repo, opted to **over-deliver** (OpenAPI/Swagger,
  Playwright, Docker) beyond the minimal brief, and insisted on **strict TDD** throughout.
- **Why this way:** a wallet is a correctness-critical domain; making the assistant justify each
  choice against sources produces a defensible design and a clean paper trail.

---

_This file is a living document — sections fill in as Steps 2 and 3 land._
