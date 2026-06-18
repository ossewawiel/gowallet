# 🪙 gowallet — Loyalty Points Wallet

> A small, production-minded backend for a **loyalty points wallet**: member accounts,
> earn/spend operations, live balances, role-based access, and safe CSV batch ingestion.
> Written in **Go**, persisted in **SQLite**.

<p align="left">
  <img alt="status" src="https://img.shields.io/badge/status-step%203%3A%20S3%20landed-blue">
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
| **3 · Design & build** | The actual wallet — 6 slices S0→S5, spec-first + TDD | 🚧 in progress |

**Where we are:**
- ✅ **Step 1** — toolchain verified, `github.com/ossewawiel/gowallet` public & pushed.
- ✅ **Step 2** — the development system: layered architecture, REST + error standards, issue-driven
  vertical-slice TDD workflow, two-layer testing (Schemathesis + Go `-race`), 3 skills, 2 subagents,
  4 commands, GitHub slice templates.
- 🚧 **Step 3** — backlog finalized (**6 slices, S0→S5**), kickoff prompts in
  [`docs/slices/`](docs/slices/), **6 GitHub issues opened** (#1–#6).
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
  - ➡️ **Next:** 🅰️ **S2 — Spend + no-negative guard** (extends the earn tx with a balance check;
    INV-3/4) · in parallel 🅲 **S4 — Audit** (INV-11) · then 🅱️ **S5 — CSV batch ingest** (INV-9/10).
  - 🔑 **Backlog +1:** 🅱️ **S6 — Login** (credential-based token issuance,
    [#10](https://github.com/ossewawiel/gowallet/issues/10)) — a real `POST /login` replacing the S3
    demo mint; seeds demo creds (see **🔑 Test credentials** below).

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

> ⏳ `POST /login` + these seeds arrive with **[S6 · login](docs/slices/S6.md)**. Until then, the S3
> demo `POST /token` mints tokens directly (no credential check).

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
