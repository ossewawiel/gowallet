# 🪙 gowallet — Loyalty Points Wallet

> A small, production-minded backend for a **loyalty points wallet**: member accounts,
> earn/spend operations, live balances, role-based access, and safe CSV batch ingestion.
> Written in **Go**, persisted in **SQLite**.

<p align="left">
  <img alt="status" src="https://img.shields.io/badge/status-step%203%3A%20S6%20landed-blue">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26.4-00ADD8">
  <img alt="db" src="https://img.shields.io/badge/SQLite-pure--Go%20(modernc)-003B57">
  <img alt="tests" src="https://img.shields.io/badge/tests-strict%20TDD-brightgreen">
</p>

---

## 📈 Progress (kept current every push)

| Step | What it delivers | State |
|------|------------------|:-----:|
| **1 · Plumbing** | Go 1.26.4 + gh + gopls, public repo live, prompt timeline started | ✅ done |
| **2 · Dev system** | CLAUDE.md, engineering docs, skills, subagents, commands, issue templates | ✅ done |
| **3 · Design & build** | The actual wallet — 8 slices S0→S7, spec-first + TDD | 🚧 in progress |

**Where we are:**
- ✅ **Step 1** — toolchain verified, `github.com/ossewawiel/gowallet` public & pushed.
- ✅ **Step 2** — the development system: layered architecture, REST + error standards, issue-driven
  vertical-slice TDD workflow, two-layer testing (Schemathesis + Go `-race`), 3 skills, 2 subagents,
  4 commands, GitHub slice templates.
- 🚧 **Step 3** — backlog grown to **8 slices (S0→S7)**, kickoff prompts in
  [`docs/slices/`](docs/slices/), a **GitHub issue per slice**.
  - ✅ **[S0 / #1](https://github.com/ossewawiel/gowallet/issues/1)** — walking skeleton **built &
    green**: `GET /healthz` (real DB ping), WAL/PRAGMAs, goose runner, `/swagger`, the 3-package
    layout wired end-to-end. Full quality gate passing. Merged to `main` via PR #7.
  - ✅ **[S1 / #2](https://github.com/ossewawiel/gowallet/issues/2)** — accounts + earn + balance
    **built & green**: `POST /accounts`, `GET /accounts/{id}`, `POST /transactions` (earn),
    `GET /accounts/{id}/balance`. Idempotent `ref` (one `sql.Tx` + single writer), derived balance,
    `kin-openapi` request validation. **INV-1/2/5/6 proven under `-race`**; full quality gate green
    (gofmt · vet · lint · build · `-race` · Schemathesis **3903 cases / 0 issues**). On branch
    `slice/s1-accounts-earn`.
  - ✅ **[S3 / #4](https://github.com/ossewawiel/gowallet/issues/4)** — auth (JWT, member/admin)
    **built & green**: stateless **HS256** (no DB, no migration), `POST /token` demo mint, algorithm
    **pinned** (`WithValidMethods`), global `security: [bearerAuth]` default. Verification is `httpapi`
    middleware; the access rule (`Authorize`) is a **pure function in `wallet`** — still 3 packages.
    S1 handlers rewired to read identity from the **verified token only**. **INV-7/8/12/13 proven**;
    full quality gate green (gofmt · vet · lint · build · `-race` · Schemathesis **1174 / 0 failures** ·
    boot fail-fast on missing secret). On branch `slice/s3-auth` (ready for PR).
  - ✅ **[S2 / #3](https://github.com/ossewawiel/gowallet/issues/3)** — spend + no-negative guard
    **built & green**: `POST /transactions` now takes `kind:"spend"`, enforcing the no-negative
    invariant **atomically** (insert-then-check-and-rollback inside one `sql.Tx`) with idempotent
    replay on duplicate `ref`. **INV-3 proven** (below-zero spend rejected → **409**) and **INV-4
    proven** (16 concurrent spends never overdraw, under `-race`). 🎉 **The earn + spend core spine
    is now complete.** On branch `slice/s2-spend` — merging to `main` via
    [PR #11](https://github.com/ossewawiel/gowallet/pull/11) (Closes #3).
  - ✅ **[S4 / #5](https://github.com/ossewawiel/gowallet/issues/5)** — audit trail **built & green**:
    a durable, **append-only** `audit_log` (the deliberate opposite of `transactions` — `ref` *not*
    unique, no FK, no kind/points CHECK, only `outcome` constrained) + an `AuditService` writer and an
    **admin-only `GET /audit`** (optional `?account_id=` filter, newest-first via `id DESC`). The money
    path is **byte-for-byte unchanged** — audit runs in its own insert, never inside the money `sql.Tx`.
    **INV-11/21/22 proven**; full quality gate green (gofmt · vet · lint **0 issues** · build · `-race`
    · Schemathesis clean incl. stateful on the new route). On branch `slice/s4-audit` (commit `fbd8d8a`,
    ready for PR).
  - ✅ **[S5 / #6](https://github.com/ossewawiel/gowallet/issues/6)** — CSV batch ingestion
    **built & green**: an admin-only **`POST /batch`** (`multipart/form-data` file upload →
    `200 BatchSummary`). Rejected rows are **data, not errors** — they're tallied in the summary
    (`processed / accepted / rejected / duplicates`); only a broken *upload* → **400**. **No
    migration, no new domain logic** — each row rides the existing `RecordEarn`/`RecordSpend`
    (idempotent via `ref` UNIQUE), the S2 atomic spend guard, and the S4 audit writer (one entry per
    row, off the money path). **INV-9/10/23 proven** (incl. a concurrent-reprocess `-race` test);
    full quality gate green (gofmt · vet · lint **0 issues** · build · `-race` · Schemathesis exit
    0). On branch `slice/s5-batch` (ready for PR). 🎉 **All core feature slices now landed.**
  - ✅ **[S6 / #10](https://github.com/ossewawiel/gowallet/issues/10)** — login **built & green**: a
    real **`POST /login`** that verifies `{account_id, secret}` against a **bcrypt** hash and issues a
    JWT carrying the account's **stored** role (a member can't self-mint admin). Unknown account and
    wrong secret return an **identical 401** (+ dummy bcrypt compare) → **no user enumeration**. The
    credential-free `POST /token` demo mint is **removed**; migration
    `20260619090000_s6_account_credentials.sql` adds `password_hash` + `role` and seeds the demo creds
    (see **🔑 Test credentials** below). **INV-14..17 proven**; full quality gate green (gofmt · vet ·
    lint **0 issues** · build · `-race` · Schemathesis **6303 cases / 0 failures**, admin token via
    `/login`). On branch `slice/s6-login`.
  - ➡️ **Next:** one slice remains —
    - 🅰️ **S7 — Listings** ([#13](https://github.com/ossewawiel/gowallet/issues/13)):
      `GET /accounts` (admin) + `GET /accounts/{id}/transactions` (own/admin). (S4 already exposes the
      admin-only `GET /audit`.)

> 🔎 Full blow-by-blow — every prompt, decision, trade-off — in
> [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md); design rationale in [`SOLUTION.md`](SOLUTION.md).

---

## 🧱 Tech stack (locked)

| Concern | Choice | Why (short) |
|---------|--------|-------------|
| Language | **Go 1.26.x** | tiny, fast, great at safe concurrency |
| Routing | **stdlib `net/http`** (1.22) **+ `chi`** | stdlib does method routing; chi adds middleware, no lock-in |
| API contract | **spec-first OpenAPI** → **`oapi-codegen`** + `kin-openapi` | `api/openapi.yaml` is the source of truth |
| Database | **SQLite** via **`modernc.org/sqlite`** | pure Go — `git clone && go run`, no C compiler |
| DB access | **`sqlc`** + **`goose`** migrations | type-safe SQL, no ORM reflection |
| Auth | **JWT HS256** (`golang-jwt`, method-pinned) | roles: `member`, `admin` |
| Testing | **Schemathesis** (contract) + **Go `-race`** (invariants) | strict TDD, two-layer source of truth |
| Packaging | **Docker** | run identically anywhere |

> 💡 Longer-term we'd swap SQLite → **PostgreSQL/MariaDB**; the layering makes that a driver change,
> not a rewrite. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

---

## 🍰 How it's built

Issue-driven **vertical slices** (one full REST cycle each), strict **red → green → refactor →
prove** TDD, proven against a two-layer source of truth. Design happens in a planning session →
a fully-specced GitHub issue → a build session ships it.

- 🏗️ Architecture & layering → [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- 🌐 REST & error conventions → [`docs/REST_API_GUIDELINES.md`](docs/REST_API_GUIDELINES.md)
- 🔄 The dev loop & quality gate → [`docs/DEVELOPMENT_FLOW.md`](docs/DEVELOPMENT_FLOW.md)
- ✅ Invariants registry (testing SoT) → [`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md)
- 🍰 Slice backlog & streams → [`docs/SLICES.md`](docs/SLICES.md)

---

## 🚀 Quickstart

> ✅ **Runnable as of S0** — the walking skeleton is live: health check, DB ping, and Swagger UI.

```bash
go run ./cmd/gowallet                      # start the API on :8080
curl localhost:8080/healthz                # -> {"status":"ok","db":"up"}
open  http://localhost:8080/swagger        # browse the live contract
```

> 🧪 Run the checks: `go test -race ./...` (needs a real gcc on the PATH for `-race` —
> e.g. MinGW; `CGO_ENABLED=1`).

---

## 🔑 Test credentials

> ⚠️ **Temporary, demo-only.** These seeded accounts exist purely for local testing and grading —
> a stop-gap so reviewers can try the protected endpoints. **Full authentication** (real user
> registration, per-user secrets, nothing published) will be implemented if more time allows.

| Role | account_id | secret |
|------|-----------|--------|
| 👤 member | `member-123` | `demo-member-pw` |
| 🛡️ admin | `admin-001` | `demo-admin-pw` |

Grab a JWT, then send it as `Authorization: Bearer <token>`:

```bash
curl -X POST localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"member-123","secret":"demo-member-pw"}'
# → { "token": "<JWT>" }
```

> ✅ `POST /login` + these seeds landed with **[S6 · login](docs/slices/S6.md)**. The old credential-
> free `POST /token` demo mint has been **removed** — a token now requires a real credential.

---

## 📂 Repository layout

```
gowallet/
├── README.md            ← you are here (progress tracked every push)
├── CLAUDE.md            ← house rules for AI sessions
├── SOLUTION.md          ← design, trade-offs, AI-workflow write-up
├── docs/                ← architecture · REST · dev-flow · acceptance · slices · prompt log
│   └── specifications.pdf   ← the original brief (the final word)
├── .claude/             ← skills · subagents · commands
├── .github/             ← issue templates
└── .vscode/             ← shared editor config (Go + gopls)
```
_(application code — `cmd/`, `internal/`, `api/`, `test/` — landed at S0 and grows per slice.)_
