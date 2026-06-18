# 🧩 SOLUTION.md — Design, Trade-offs & AI Workflow

Plain-language tour of **why gowallet is built the way it is**, what's actually shipped, the
trade-offs we made, and **how AI was used** (a requirement of the brief). The blow-by-blow timeline
lives in [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md) — this is the curated summary.

---

## 1. What it is, in one paragraph

gowallet is a loyalty **points wallet**: members earn and spend points, and the service keeps
balances correct even when the same transaction is submitted twice or many requests land at once.
The guiding principle is **boring correctness over clever code** — a clear data model, balances
derived from an append-only transaction log, idempotent writes keyed on the transaction `ref`, and
SQL-level guards that can't be torn apart by concurrency. It was built in deliberate stages
(plumbing → dev system → issue-driven slices) so the *reasoning* is visible, not just the artifact.

---

## 2. Where it stands (built vs planned)

| Slice | Capability | State |
|------|------------|:-----:|
| **S0** | Walking skeleton — `/healthz`, DB PRAGMAs, goose runner, Swagger UI, 3-package wiring | ✅ built |
| **S1** | Accounts + earn + balance — `POST /accounts`, `GET /accounts/{id}`, `POST /transactions` (earn), `GET …/balance` | ✅ built |
| **S2** | Spend + atomic no-negative guard | ✅ built |
| **S3** | Auth — JWT HS256 verify middleware + member/admin access rule | ✅ built |
| **S4** | Audit trail — append-only `audit_log` + admin `GET /audit` | ✅ built |
| **S5** | CSV batch ingestion — admin `POST /batch` + summary | ✅ built |
| **S6** | Login — credential-based token issuance | 📋 specced (issue #10) |
| **S7** | Listings — `GET /accounts`, `GET /accounts/{id}/transactions` | 📋 specced (issue #13) |

All six **core feature slices (S0–S5) are built and merged** — accounts, earn, spend, auth, audit,
and CSV batch ingestion. Only **S6 (login)** and **S7 (listings)** remain; both are fully designed
in [`docs/slices/`](docs/slices/) with GitHub issues ready to build.

---

## 3. Architecture (three packages, one-way deps)

A light hexagonal layout so the business rules never touch HTTP or SQL — which is what makes the
"swap SQLite for Postgres later" promise real. Full detail in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

```
httpapi  ──▶  wallet  ◀──  sqlitestore
(transport)   (THE CORE)   (persistence)
```

- **`internal/wallet`** — domain types, balance + idempotency rules, services, the `Authorize`
  access rule, sentinel errors. Imports nothing internal; unit-testable with no DB.
- **`internal/sqlitestore`** — sqlc-generated queries, goose migrations, DB open + PRAGMAs.
  Implements `wallet`'s repository interfaces.
- **`internal/httpapi`** — oapi-codegen handlers, JWT middleware, the router, the error envelope.
- **`cmd/gowallet/main.go`** — the one place that wires all three together.

Auth and CSV are *not* their own packages — auth is `httpapi` middleware, CSV is an `httpapi`
handler calling `wallet`. Three packages, full stop.

---

## 4. The API & data model (as built)

**Endpoints live today** (`api/openapi.yaml` is the source of truth; `/swagger` serves it):

| Method + path | Does | Auth |
|---------------|------|------|
| `GET /healthz` | liveness + real DB ping | public |
| `POST /token` | mint a demo HS256 JWT for `{account_id, role}` | public |
| `POST /accounts` | create a member account | Bearer |
| `GET /accounts/{id}` | read one account | Bearer (own/admin) |
| `GET /accounts/{id}/balance` | current balance | Bearer (own/admin) |
| `POST /transactions` | record earn **or** spend (idempotent on `ref`) | Bearer (own/admin) |
| `POST /batch` | admin CSV upload → summary (processed / accepted / rejected / duplicates) | Bearer (admin) |
| `GET /audit` | admin view of the append-only audit log (`?account_id=` filter) | Bearer (admin) |

**Data model** (two tables; balances are *derived*, never stored):

```sql
accounts(account_id PK, name, created_at)
transactions(id PK, ref UNIQUE, account_id FK→accounts, kind CHECK(earn|spend),
             points CHECK(>0), occurred_at, created_at)   -- index on account_id
```

The whole stack is real and current: `chi v5.3`, `kin-openapi v0.140`, `oapi-codegen` runtime,
`golang-jwt/jwt v5.3`, `goose v3.27`, `modernc.org/sqlite v1.52` — all pinned in `go.mod`.

---

## 5. Key decisions & trade-offs

The full reasoning + primary sources are in [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md).

| # | Decision | Alternatives | Why |
|---|----------|--------------|-----|
| D1 | **SQLite** | PostgreSQL, MariaDB | brief-recommended; zero external services; data layer kept swappable |
| D2 | **`modernc.org/sqlite`** (pure Go) | `mattn/go-sqlite3` (CGO) | no C compiler → `git clone && go run` anywhere; minor write-speed cost is irrelevant here |
| D3 | **Single on-disk `.db`** | in-memory, server DB | "durable across restarts" with the fewest moving parts |
| D4 | **Public repo** | private + invite | simplest to share/review |
| D5 | **Spec-first OpenAPI + `oapi-codegen`** + `kin-openapi` | code-first (Huma/swaggo), hand-rolled | `openapi.yaml` is the single source of truth **and** the TDD target |
| D6 | **stdlib `net/http` (1.22) + `chi`** | gin / echo / fiber | stdlib does method routing; chi adds middleware without framework lock-in |
| D7 | **`sqlc`** + **`goose`** (timestamped) | GORM, sqlx | type-safe SQL, zero reflection; timestamped migrations survive parallel branches |
| D8 | **JWT HS256**, method-pinned | opaque, PASETO, RS256 | single service signs **and** verifies → symmetric is right; `WithValidMethods` kills alg-confusion |
| D9 | **Schemathesis + Go `-race`** | Playwright, Dredd | two-layer source of truth: spec-fuzz for shapes, race tests for business rules + concurrency |
| D10 | **Derived balance** (`Σearn − Σspend`) | stored running-balance column | no dual-write; "balance persists across restart" falls out for free |
| D11 | **Strict TDD**, issue-driven vertical slices, 3 streams | one-shot build, test-after | correctness is the whole point; each issue fully specs a slice so a fresh session builds with no re-design |
| D12 | **Docker** for portable runs | local-only | runs anywhere without a local Go install |
| D13 | **Thin credential login** (S6) over a full IdP | demo-mint only / full user mgmt | answers "how do you log in?" + shows authn-vs-authz, without scope creep |

> ✏️ Course corrections worth flagging: we **dropped Playwright** (Schemathesis owns the contract
> path); **flattened** a 5-package layout to 3 (the Go team favours simpler); **collapsed** accounts
> + earn into one slice; and **baked concurrency tests into** S1/S2 rather than a separate hardening
> slice. Each is logged with its reason in the prompt log.

---

## 6. Correctness model (the part that matters — and how it's actually done)

| Constraint (brief) | How it's implemented |
|--------------------|----------------------|
| **No double-counting a `ref`** | `UNIQUE(ref)` + `INSERT … ON CONFLICT(ref) DO NOTHING` inside one `sql.Tx`. `RowsAffected == 1` → created (**201**); `== 0` → idempotent replay returns the stored txn (**200**). |
| **No spend below zero** | Within the **same tx**: insert the spend, then recompute the balance (which now *includes* it) and **roll back** if it came out `< 0` → **409**. The check and the write can't be separated. |
| **Safe under overlapping requests** | The write path pins **`SetMaxOpenConns(1)`** (single writer) + **WAL** + `busy_timeout=5000`, so racing spends each see every committed prior spend. Concurrency holds *by construction*, not by hope. |
| **Durable across restarts** | On-disk SQLite, `journal_mode=WAL`, `synchronous=NORMAL`. Balance is a `SUM` over surviving rows. |
| **No wire-crossing** | Only the `*sql.DB` pool + config are shared; identity rides in `r.Context()` from the **verified token only**. Proven by an N-user `-race` isolation test. |
| **Batch safe on reprocess** | Each CSV row rides the *same* idempotent `RecordEarn`/`RecordSpend` (`UNIQUE(ref)`); reprocessing the same file double-counts nothing. Rejected rows are tallied as data, not errors. |
| **Audit of every attempt** | An **append-only** `audit_log` (ref *not* unique, written in its own insert — never inside the money `sql.Tx`) records each attempt's outcome + reason; admin reads via `GET /audit`. |

Every row above is backed by a test in [`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md) — the invariant
registry. **INV-1–13 and 21–23 are proven (green under `-race`)** — accounts, earn, spend, auth,
audit, and batch. Only **INV-14–20** remain (S6 login, S7 listings).

---

## 7. Security model

- **Verification:** Bearer JWT, **HS256**, algorithm **pinned** (`jwt.WithValidMethods(["HS256"])`)
  → defeats the `alg:none` and RS↔HS confusion attacks. Claims: `sub` (account_id) + `role`.
- **Authorization:** a **pure function** `wallet.Authorize(identity, target)` — `member` may only
  touch their own account (else **403**), `admin` may touch any. Identity comes from the token via
  context, **never** the URL/body.
- **Spec default-deny:** `openapi.yaml` sets a global `security: [bearerAuth]`; public routes opt out
  explicitly. So any new endpoint is **protected by default**.
- **Config:** `GOWALLET_JWT_SECRET` is required and the server **fails fast at boot** if it's missing.

> ⚠️ **Honest caveat:** today `POST /token` is a *demo mint* — it issues a token without checking a
> credential (in-spec: the brief leaves issuance to us). **S6 (login)** replaces it with a real
> `POST /login` that verifies a bcrypt-hashed secret; its seeded demo credentials are published in
> the README purely for testing and flagged as a temporary stop-gap.

---

## 8. Testing strategy

Two layers, because no single tool covers all of it:

1. **Contract (Schemathesis):** property-based + stateful fuzzing straight from `openapi.yaml` —
   catches shape violations, bad status codes, and sequence bugs. (S1 ran **3903 cases / 0 issues**.)
2. **Invariants (Go `testing` + `-race`):** the business rules + concurrency the spec can't express —
   idempotency, no-negative, isolation. Run with the race detector.

Strict **red → green → refactor → prove** per slice; "done" = the full quality gate green (`gofmt`,
`go vet`, `golangci-lint`, `go build`, `go test -race`, Schemathesis, the slice's ACCEPTANCE rows,
and updated docs).

> 🪟 One Windows wrinkle worth knowing: the pure-Go driver means normal builds are CGO-free, but
> `go test -race` itself needs cgo + a real **MinGW gcc** — documented in `CLAUDE.md` so it isn't
> rediscovered each time.

---

## 9. Trade-offs & what we'd do with more time

- **Token issuance** is a demo mint until S6 lands; published test creds are a temporary grading aid.
- **No pagination** on the planned listings (S7) — fine at demo scale; production wants limit/offset
  or cursor paging.
- **SQLite single-writer** is perfect here; higher write concurrency would mean the Postgres swap
  (a driver + migration change, not a rewrite — that's why the layering exists).
- **`int64` balance overflow** is flagged as a known edge (noted during the S3 build) to be handled
  where it belongs.
- **Only login (S6) + listings (S7)** remain unbuilt — the audit trail and CSV batch ingestion are
  merged to `main`.

---

## 10. 🤖 AI workflow — what was asked, accepted, edited, and why

The brief asks for an honest account of how AI was used. Full per-decision timeline in
[`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md); here's the shape of it.

**How AI was used — as an interrogator + pair-engineer, not an autocomplete.** Every phase started
with a hard-questions pass (an "interrogation" that surfaced trade-offs and cited primary sources —
go.dev, sqlite.org, the IETF idempotency draft, Russ Cox on Go layout) *before* any code. Then we
built a small **factory**: house rules in `CLAUDE.md`, three skills, two subagents, four slash
commands, and a GitHub **issue-per-slice** workflow — so design happens in a planning session, lands
as a fully-specced issue, and a fresh session builds it red→green→refactor→prove with no re-design.

- **✅ Accepted (AI recommendations taken):** pure-Go SQLite driver (portability); spec-first
  `oapi-codegen`; `sqlc`; **JWT HS256 over RS256** (single service — no key split to justify);
  WAL + `busy_timeout` + single-writer for concurrency; the two-layer Schemathesis + `-race` testing.
- **✏️ Steered/edited (where I overrode or shaped it):** public repo; **over-deliver** scope
  (OpenAPI/Swagger, Docker, CI); strict TDD always; a **casual, visual house voice** applied to all
  docs; a **granular** prompt log; flattening to a **3-package** layout; collapsing accounts+earn;
  adding the **thin login** (S6) and **listings** (S7) after spotting gaps.
- **❌ Rejected (AI options declined, with reasons):** gin/echo/GORM (framework/ORM lock-in);
  Playwright on the contract path (Schemathesis is better at it); the IETF `Idempotency-Key` header
  (overkill — body `ref` is the key); a 5-package layout; full user-management (scope creep).
- **💡 Why this way:** a wallet is correctness-critical. Forcing every choice to be justified against
  sources — and capturing it as a timeline — produces a defensible design *and* a clean paper trail,
  which is exactly what the brief grades.

---

_Living document — synced to `main` every time a slice lands (see `.claude/agents/doc-updater.md` +
the quality gate). Last synced after **S5 landed**: audit + batch are on `main`; **S6 login + S7
listings** remain._
