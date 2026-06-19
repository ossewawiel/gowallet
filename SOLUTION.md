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
| **S6** | Login — credential-based `POST /login` (bcrypt-hashed secrets, role from store) | ✅ built |
| **S7** | Listings — `GET /accounts`, `GET /accounts/{id}/transactions` | ✅ built |
| **S8** | Redeem — member `POST /accounts/{id}/redeem` for a reward (deducts, counts against balance) | ✅ built |

**S0–S8 are all built** — accounts, earn, spend, auth, audit, CSV batch ingestion, real
credential login, the read/collection listings, **and member point-redemption**. The whole feature
backlog has landed ([issue #19](https://github.com/ossewawiel/gowallet/issues/19) closed the loop);
remaining work is hardening/polish, not new features.

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
| `POST /login` | verify `{account_id, secret}` (bcrypt) → HS256 JWT carrying the **stored** role | public |
| `POST /accounts` | create a member account (optional `secret`, stored bcrypt-hashed) | Bearer |
| `GET /accounts/{id}` | read one account | Bearer (own/admin) |
| `GET /accounts/{id}/balance` | current balance | Bearer (own/admin) |
| `POST /transactions` | record earn **or** spend (idempotent on `ref`) | Bearer (own/admin) |
| `POST /accounts/{id}/redeem` | redeem points for a `reward` — deducts, idempotent on `ref` (**201** new / **200** replay), **409** if balance can't cover it | Bearer (own/admin) |
| `POST /batch` | admin CSV upload → summary (processed / accepted / rejected / duplicates) | Bearer (admin) |
| `GET /audit` | admin view of the append-only audit log (`?account_id=` filter) | Bearer (admin) |
| `GET /accounts` | admin list of **all** accounts, each with its derived balance | Bearer (admin) |
| `GET /accounts/{id}/transactions` | per-account ledger, newest-first; **404** on a ghost account | Bearer (own/admin) |

**Data model** (two tables; balances are *derived*, never stored):

```sql
accounts(account_id PK, name, password_hash, role CHECK(member|admin) DEFAULT 'member', created_at)
transactions(id PK, ref UNIQUE, account_id FK→accounts, kind CHECK(earn|spend|redeem),
             points CHECK(>0), reward TEXT NULL, occurred_at, created_at)   -- index on account_id
```

> 🔐 `password_hash` is nullable (only credentialed accounts can log in) and holds a **bcrypt** hash —
> the plaintext `secret` is `writeOnly` and never returned in any response.

> 🎁 **S8 widened `transactions`.** A timestamped *rebuild* migration
> (`20260619100000_s8_redeem_kind.sql`, create→copy→drop→rename — SQLite can't `ALTER` a `CHECK`)
> widened `kind` to include **`redeem`** and added a nullable **`reward` TEXT** column, preserving
> `UNIQUE(ref)`, the FK, the `points>0` CHECK and `idx_transactions_account`. It's FK-safe inside
> goose's default transaction (nothing FK-references `transactions`), so no PRAGMA toggle was needed;
> up/down round-trip verified. `reward` rides the redeem response + the stored row only — the
> per-account ledger shape is unchanged.

> 📖 **S7 added no migration.** The listings read straight over the existing `accounts` +
> `transactions` tables via two new sqlc queries: **`ListAccountsWithBalance`** (a correlated
> subquery that reuses the *exact* same balance formula as `BalanceForAccount`, so a list row
> can never disagree with `GET /balance`) and **`ListTransactionsByAccount`** (ordered `id DESC` for
> a stable newest-first ledger, riding the existing `idx_transactions_account` index).

> 🧮 **S8 made the deduction set explicit.** The balance formula in both `BalanceForAccount` and
> `ListAccountsWithBalance` now reads `WHEN kind IN ('spend','redeem') THEN -points ELSE 0` (it used
> to be a catch-all `ELSE -points`). The old catch-all *already* deducted a `redeem` correctly — the
> real risk was the over-eager `ELSE`, so naming the deduction set means a *future* kind can't be
> silently treated as a deduction. Balance is now **Σ(earn) − Σ(spend) − Σ(redeem)**.

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
| **No redeem below zero** | A **redeem** reuses the *exact same* atomic in-tx guard as spend — the guard's trigger was simply widened to `spend OR redeem`, and it reuses the **same** `ErrInsufficientBalance` sentinel (no new error). Insert-then-recompute-then-rollback-if-`<0`; racing redeems (and redeem-vs-spend races) can't overdraw. Redeem carries a required `reward`, stored on the row + returned. |
| **Safe under overlapping requests** | The write path pins **`SetMaxOpenConns(1)`** (single writer) + **WAL** + `busy_timeout=5000`, so racing spends each see every committed prior spend. Concurrency holds *by construction*, not by hope. |
| **Durable across restarts** | On-disk SQLite, `journal_mode=WAL`, `synchronous=NORMAL`. Balance is a `SUM` over surviving rows. |
| **No wire-crossing** | Only the `*sql.DB` pool + config are shared; identity rides in `r.Context()` from the **verified token only**. Proven by an N-user `-race` isolation test. |
| **Batch safe on reprocess** | Each CSV row rides the *same* idempotent `RecordEarn`/`RecordSpend` (`UNIQUE(ref)`); reprocessing the same file double-counts nothing. Rejected rows are tallied as data, not errors. |
| **Audit of every attempt** | An **append-only** `audit_log` (ref *not* unique, written in its own insert — never inside the money `sql.Tx`) records each attempt's outcome + reason; admin reads via `GET /audit`. |
| **Listings leak nothing** | `GET /accounts` reuses **`requireAdmin`** (same gate as `GET /audit`) — members get **403**. The ledger reuses **`authorizeTarget`/`wallet.Authorize`** (member-own / admin-any) and **authorizes BEFORE hitting the store** — so a cross-account member gets a clean **403** (not a 404 that would leak whether the account exists), while an admin on a ghost account surfaces an honest **404**. Identity always from the verified token, never the URL. |

Every row above is backed by a test in [`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md) — the invariant
registry. **INV-1–28 are now all proven (green under `-race`)** — accounts, earn, spend, auth,
audit, batch, login, listings, **and redeem** (INV-24/25/26/27/28 landed with S8, including the
redeem-vs-spend concurrency race).

---

## 7. Security model

- **Verification:** Bearer JWT, **HS256**, algorithm **pinned** (`jwt.WithValidMethods(["HS256"])`)
  → defeats the `alg:none` and RS↔HS confusion attacks. Claims: `sub` (account_id) + `role`.
- **Authorization:** a **pure function** `wallet.Authorize(identity, target)` — `member` may only
  touch their own account (else **403**), `admin` may touch any. Identity comes from the token via
  context, **never** the URL/body.
- **Spec default-deny:** `openapi.yaml` sets a global `security: [bearerAuth]`; public routes opt out
  explicitly. So any new endpoint is **protected by default**.
- **Listings reuse the same gates (no new authz):** `GET /accounts` rides `requireAdmin`; the
  per-account ledger rides `authorizeTarget`/`wallet.Authorize` and **authorizes before** touching
  the store, so a cross-account member gets **403** rather than a 404 that would leak existence.
- **Authentication (login):** `POST /login` verifies `{account_id, secret}` against a **bcrypt** hash
  and issues a JWT carrying the account's **stored** `role` (never the request) — a member can't
  self-mint admin. Unknown account and wrong secret return an **identical 401** (+ a dummy bcrypt
  compare on the miss path) so there's **no user enumeration** by response shape *or* timing.
- **Config:** `GOWALLET_JWT_SECRET` is required and the server **fails fast at boot** if it's missing.

> ✅ **Resolved caveat:** the old credential-free `POST /token` demo mint has been **removed** —
> **S6 (login)** replaced it with the real `POST /login` above. Seeded demo credentials are published
> in the README purely for testing and flagged as a temporary stop-gap until full user management.

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

- **Token issuance** is now a real credential login (S6 landed); published test creds are a temporary
  grading aid until full user management (registration/reset).
- **No pagination** on the listings (S7) — every row is returned, which is fine at demo scale. The
  **production upgrade** is `limit`/`offset` or a keyset/cursor on `transactions.id`; because the
  per-account ledger already rides the existing `idx_transactions_account` index, cursor paging is a
  drop-in (no schema or query-shape rework).
- **SQLite single-writer** is perfect here; higher write concurrency would mean the Postgres swap
  (a driver + migration change, not a rewrite — that's why the layering exists).
- **`int64` balance overflow** is flagged as a known edge (noted during the S3 build) to be handled
  where it belongs.
- **The full feature backlog (S0–S8) is now merged** — accounts, earn, spend, auth, audit, CSV
  batch, login, listings, and redeem. Remaining work is hardening/polish (see
  [`docs/SLICES.md`](docs/SLICES.md)).

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
the quality gate). Last synced after **S8 (redeem) landed** on 2026-06-19: added `POST
/accounts/{id}/redeem` (own/admin), a rebuild migration widening `transactions.kind` to include
`redeem` + a nullable `reward` column, an explicit `kind IN ('spend','redeem')` deduction set, and
reuse of the S2 atomic guard + `ErrInsufficientBalance` (no new error). INV-24–28 proven. **All
feature slices S0–S8 are now shipped.**_
