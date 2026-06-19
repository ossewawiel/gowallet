# 🪙 gowallet — Loyalty Points Wallet

> A small, production-minded backend for a **loyalty points wallet**: member accounts, earn/spend
> operations, live balances, role-based access, CSV batch ingest, and an audit trail — with balances
> that **stay correct under duplicates and concurrency**. Built in **Go**, persisted in **SQLite**.

<p align="left">
  <img alt="status" src="https://img.shields.io/badge/slices-S0%E2%80%93S8%20shipped-success">
  <img alt="gate" src="https://img.shields.io/badge/quality%20gate-green-brightgreen">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26.x-00ADD8">
  <img alt="db" src="https://img.shields.io/badge/SQLite-pure--Go%20(modernc)-003B57">
  <img alt="tests" src="https://img.shields.io/badge/tests-strict%20TDD%20%C2%B7%20%2Drace%20%2B%20Schemathesis-brightgreen">
</p>

---

## 🎯 What it is

gowallet is a REST API where **accounts earn and spend loyalty points** and the balance is always
right — even when the same request arrives twice or two requests race. The hard parts are the
*correctness guarantees*, and they're enforced where they can't be bypassed:

| Guarantee | How it's enforced |
|-----------|-------------------|
| 🔁 **Idempotency** — the same `ref` counts **once** | `UNIQUE(ref)` + insert-or-replay in one `sql.Tx` |
| 🚫 **No negative balance** — a spend can't overdraw | balance check **inside the same transaction** as the write; rollback if it'd go below zero |
| ⚡ **Concurrency-safe** — racing spends never overdraw | single writer serialises the money path; proven by `-race` tests |
| 🧍 **No wire-crossing** — users only ever see their own data | identity rides `r.Context()` from a **verified token**, never the URL; nothing request-specific is shared |
| 🔐 **Role-based access** — `member` (own only) vs `admin` (any) | a pure `Authorize` rule, sourced from the JWT's `role` claim |
| 🧾 **Auditable** — every attempt recorded with outcome + reason | append-only `audit_log`, written **off** the money path |

> Points are **integers** (never floats). Money rules live at the **SQL level**, not in app code that
> could be raced around. The full design rationale is in [`SOLUTION.md`](SOLUTION.md).

---

## 🚀 Get it running

> Pure-Go SQLite means **no C compiler to run it** — `git clone && go run`.

```bash
export GOWALLET_JWT_SECRET="demo-secret-please-change"   # required — boot fails loud without it
go run ./cmd/gowallet                                     # → gowallet listening on :8080

curl -s localhost:8080/healthz                            # → {"status":"ok","db":"up"}
open  http://localhost:8080/swagger                       # browse the live OpenAPI contract
```

First boot runs the migrations and **seeds two demo accounts**. Grab a token and start poking:

| Role | `account_id` | `secret` |
|------|--------------|----------|
| 👤 member | `member-123` | `demo-member-pw` |
| 🛡️ admin | `admin-001` | `demo-admin-pw` |

```bash
curl -s -X POST localhost:8080/login -H 'Content-Type: application/json' \
  -d '{"account_id":"admin-001","secret":"demo-admin-pw"}'        # → { "token": "<JWT>" }
# then send it on protected routes:  -H "Authorization: Bearer <JWT>"
```

> 🎬 **Full guided tour** — every endpoint, earn/spend, the spend guard, batch, audit, the access
> split — copy-paste in **[`docs/DEMO.md`](docs/DEMO.md)**. (Demo creds are a stop-gap for
> grading; real registration is the next auth step.)

---

## 🌐 The API at a glance

Spec-first: `api/openapi.yaml` is the source of truth, served live at `/openapi.yaml`. Every route
returns the **same JSON error envelope** (`{ "error": { code, message, request_id } }`).

| Method + path | Auth | What it does |
|---------------|------|--------------|
| `GET /healthz` | 🌍 public | liveness + DB readiness |
| `POST /login` | 🌍 public | credential → signed **HS256** JWT (role from the **stored** account) |
| `POST /accounts` | 🔓 any token | create a member account (optional login secret) |
| `GET /accounts` | 🛡️ admin | list **all** accounts + derived balance |
| `GET /accounts/{id}` | 🧍 own / admin | read one account |
| `GET /accounts/{id}/balance` | 🧍 own / admin | current balance (live Σ earn − Σ spend) |
| `GET /accounts/{id}/transactions` | 🧍 own / admin | that account's ledger, **newest-first** |
| `POST /transactions` | 🧍 own / admin | record an **earn**/**spend** (idempotent on `ref`) |
| `POST /accounts/{id}/redeem` | 🧍 own / admin | redeem points for a **reward** — deducts, counts against balance, idempotent on `ref` |
| `POST /batch` | 🛡️ admin | ingest a CSV → `{processed, accepted, rejected, duplicates}` |
| `GET /audit` | 🛡️ admin | the audit log, newest-first (optional `?account_id=`) |
| `GET /openapi.yaml`, `GET /swagger` | 🌍 public | the live contract + Swagger UI |

**Status codes** are honest: `201` create · `200` read/replay · `400/422` bad input · `401` no/bad
token · `403` not allowed · `404` missing · `409` would-go-negative. → [`docs/REST_API_GUIDELINES.md`](docs/REST_API_GUIDELINES.md)

---

## 🧪 Testing — two layers, one source of truth

Correctness is pinned from **two angles** so nothing slips between them:

| Layer | Proves | Tool | Command |
|-------|--------|------|---------|
| **Contract** | every operation matches the spec — status, shape, realistic sequences | **Schemathesis** (property + stateful) | fuzzes `/openapi.yaml` |
| **Invariants** | business rules, **concurrency**, no wire-crossing | **Go `testing` + `-race`** | `go test -race ./...` |

The business invariants (idempotency, no-overdraw, no cross-user leak) live in a named registry —
[`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md) — where each `INV-n` maps to a test. **All 28 are green** —
`INV-24–28` landed with the redeem slice (S8), including the redeem-vs-spend concurrency race.

```bash
CGO_ENABLED=1 go test -race ./...     # -race needs cgo + a real gcc (e.g. MinGW) on the PATH
```

> 🪟 **Windows toolchain paths** (Go, gcc, Schemathesis, oapi-codegen, sqlc) and the exact
> Schemathesis-with-token recipe live in [`CLAUDE.md`](CLAUDE.md). Full test walkthrough →
> [`docs/DEMO.md` §9](docs/DEMO.md).

**Latest full gate:** gofmt ✓ · vet ✓ · golangci-lint **0 issues** ✓ · build ✓ · `-race` ✓ ·
Schemathesis **10104 cases · exit 0** ✓.

---

## 🏗️ Architecture in one breath

Three internal packages, dependencies pointing **one way** — both edges aim at `wallet`, which
imports neither:

```
  httpapi  ─────▶  wallet  ◀─────  sqlitestore
 (transport,      (pure rules,     (sqlc + goose
  JWT, CSV)        no DB, no HTTP)   + the *sql.DB pool)
```

- `wallet` is **pure Go** — unit-testable with no database, and it owns the access rule + sentinel errors.
- `httpapi` never imports `sqlitestore`; `cmd/gowallet/main.go` is the **only** place all three are wired.
- The **one** thing shared across requests is the `*sql.DB` pool (+ config). Everything else rides the
  request context — that's *why* wire-crossing is structurally impossible.

→ Layering, the SQLite PRAGMAs, and the swap-to-Postgres path: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

---

## 🤖 How it's built — the AI workflow

This repo is also a demonstration of a **disciplined AI build loop**. Work moves in **vertical
slices** (one full REST cycle each), strict **red → green → refactor → prove** TDD, every slice
driven by a GitHub issue. The discipline is encoded as Claude Code tooling so it can't be skipped:

### ⌨️ Commands — *you type these* (`.claude/commands/`)

| Command | Purpose |
|---------|---------|
| **`/design-slice <id>`** | Design a slice (REST cycle, OpenAPI fragment, migration, invariants, red-test list) and open a **fully-specced GitHub issue** — *design only, no code*. |
| **`/build-slice <issue#>`** | Build that slice via the `tdd-runner` subagent: branch → 🔴 red → 🟢 green → 🧹 refactor → 🧪 prove. |
| **`/quality-gate`** | Run the whole gate (fmt · vet · lint · build · `-race` · Schemathesis) and report pass/fail. |
| **`/log-progress`** | Append a curated entry to the prompt/decision timeline via `doc-updater`. |

### 📐 Skills — *auto-pulled house rules* (`.claude/skills/`)

| Skill | What it enforces |
|-------|------------------|
| **`go-architecture`** | the 3-package layering, the wire-crossing rule, SQLite/sqlc/goose patterns, error design. |
| **`rest-api-standards`** | resource naming, status codes, the single error envelope, idempotency via `ref`, JWT/roles, spec-first. |
| **`tdd-workflow`** | spec-first red/green/refactor, the two-layer testing model, the quality gate, the acceptance registry. |

### 🧩 Subagents — *isolated context* (`.claude/agents/`)

| Subagent | Job |
|----------|-----|
| **`tdd-runner`** | Builds one slice end-to-end in its own context window; returns a concise pass/fail — keeps the main session lean. |
| **`doc-updater`** | Keeps the living docs current: [`PROMPT_LOG.md`](docs/PROMPT_LOG.md), [`SOLUTION.md`](SOLUTION.md), [`ACCEPTANCE.md`](docs/ACCEPTANCE.md), this README. |

### 🔄 The loop

```
  /design-slice  ──▶  GitHub issue (the spec)  ──▶  /build-slice ──▶  tdd-runner
       │                                                                  │
       │                                            red → green → refactor → prove (quality gate)
       ▼                                                                  ▼
  doc-updater registers invariants                       doc-updater syncs docs ──▶ PR closes the issue
```

Two **enforced gates** keep it honest: a `pre-push` hook blocks any push that changes the API spec or
production Go without touching `SOLUTION.md`, and a slice isn't "done" until the whole quality gate is
green. → [`docs/DEVELOPMENT_FLOW.md`](docs/DEVELOPMENT_FLOW.md) · [`docs/SLICES.md`](docs/SLICES.md).

---

## 📈 Progress

| Step | What it delivers | State |
|------|------------------|:-----:|
| **1 · Plumbing** | Go toolchain, public repo, prompt timeline started | ✅ done |
| **2 · Dev system** | CLAUDE.md, engineering docs, skills, subagents, commands, issue templates | ✅ done |
| **3 · Design & build** | the wallet — **9 slices S0→S8**, spec-first + TDD | ✅ **all shipped** |

**Just landed: `S8` redeem** — `POST /accounts/{id}/redeem` deducts points for a reward, reusing the
S2 atomic guard (no overdraw, even on redeem-vs-spend races) and idempotent on `ref`.

**All feature slices are landed and green** — `S0` skeleton · `S1` accounts/earn/balance · `S2`
spend + no-negative guard · `S3` JWT auth · `S4` audit trail · `S5` CSV batch · `S6` credential
login · `S7` listings · `S8` redeem. Every `INV-1…28` proven; full gate green.

➡️ **Remaining is hardening/polish**, not new features: pagination on the listings, integer-overflow
handling, the Postgres-swap path.

<details>
<summary>📜 Per-slice detail & issue links</summary>

- ✅ **[S0 / #1](https://github.com/ossewawiel/gowallet/issues/1)** — walking skeleton: `/healthz`, WAL/PRAGMAs, goose runner, `/swagger`, the 3-package layout.
- ✅ **[S1 / #2](https://github.com/ossewawiel/gowallet/issues/2)** — accounts + earn + balance; idempotent `ref`, derived balance. INV-1/2/5/6.
- ✅ **[S2 / #3](https://github.com/ossewawiel/gowallet/issues/3)** — spend + atomic no-negative guard. INV-3/4 (16 concurrent spends, no overdraw).
- ✅ **[S3 / #4](https://github.com/ossewawiel/gowallet/issues/4)** — JWT HS256 auth, algorithm pinned, `Authorize` a pure `wallet` function. INV-7/8/12/13.
- ✅ **[S4 / #5](https://github.com/ossewawiel/gowallet/issues/5)** — append-only `audit_log` + admin `GET /audit`, off the money path. INV-11/21/22.
- ✅ **[S5 / #6](https://github.com/ossewawiel/gowallet/issues/6)** — admin `POST /batch` CSV ingest; rejects are data, not errors. INV-9/10/23.
- ✅ **[S6 / #10](https://github.com/ossewawiel/gowallet/issues/10)** — `POST /login`, bcrypt creds, role from store, identical-401 (no enumeration). INV-14–17.
- ✅ **[S7 / #13](https://github.com/ossewawiel/gowallet/issues/13)** — admin `GET /accounts` + per-account ledger; reuse, no migration. INV-18/19/20.
- ✅ **[S8 / #19](https://github.com/ossewawiel/gowallet/issues/19)** — `POST /accounts/{id}/redeem` for a reward; `kind=redeem`, reuses the S2 atomic guard, explicit deduction set. INV-24–28.

</details>

> 🔎 Blow-by-blow of every prompt & decision → [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md).

---

## 🧱 Tech stack (locked)

| Concern | Choice | Why (short) |
|---------|--------|-------------|
| Language | **Go 1.26.x** | tiny, fast, great at safe concurrency |
| Routing | **stdlib `net/http`** (1.22) **+ `chi`** | stdlib method routing; chi adds middleware, no lock-in |
| API contract | **spec-first OpenAPI** → **`oapi-codegen`** + `kin-openapi` | `api/openapi.yaml` leads, code follows |
| Database | **SQLite** via **`modernc.org/sqlite`** | pure Go — no C compiler to run |
| DB access | **`sqlc`** + **`goose`** migrations | type-safe SQL, no ORM reflection |
| Auth | **JWT HS256** (`golang-jwt`, method-pinned) | roles: `member`, `admin` |
| Testing | **Schemathesis** (contract) + **Go `-race`** (invariants) | two-layer source of truth |

> 💡 SQLite → **PostgreSQL/MariaDB** later is a *driver* change, not a rewrite — that's what the
> layering buys. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

---

## 📂 Repository layout

```
gowallet/
├── README.md            ← you are here (the front door)
├── CLAUDE.md            ← house rules for AI sessions + Windows toolchain paths
├── SOLUTION.md          ← design, trade-offs, AI-workflow write-up (graded)
├── api/openapi.yaml     ← the API contract (source of truth)
├── cmd/gowallet/        ← main.go — the one place the 3 packages are wired
├── internal/
│   ├── httpapi/         ← transport: router, JWT middleware, handlers, CSV, error envelope
│   ├── wallet/          ← pure core: rules, services, Authorize, sentinel errors
│   └── sqlitestore/     ← sqlc queries + goose migrations + the *sql.DB pool
├── test/acceptance/     ← Layer-2 invariant tests (-race)
├── docs/                ← architecture · REST · dev-flow · acceptance · slices · DEMO · prompt log
│   └── specifications.pdf   ← the original brief (the final word)
├── .claude/             ← commands · skills · subagents
└── .github/             ← issue templates
```

---

## 🗂️ Docs index

| Doc | What's in it |
|-----|--------------|
| [`docs/DEMO.md`](docs/DEMO.md) | 🎬 guided curl tour of the whole API + how to run the tests |
| [`SOLUTION.md`](SOLUTION.md) | 🧠 design, trade-offs, the AI-workflow write-up |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 🏗️ layering, the wire-crossing rule, DB patterns |
| [`docs/REST_API_GUIDELINES.md`](docs/REST_API_GUIDELINES.md) | 🌐 endpoints, status codes, the error envelope |
| [`docs/DEVELOPMENT_FLOW.md`](docs/DEVELOPMENT_FLOW.md) | 🔄 the slice loop + quality gate |
| [`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md) | ✅ the invariant registry (testing source of truth) |
| [`docs/SLICES.md`](docs/SLICES.md) | 🍰 the slice backlog & parallel streams |
| [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md) | 📜 every prompt, decision, and trade-off |
