# 🪙 gowallet — Loyalty Points Wallet

> A small, production-minded backend for a **loyalty points wallet**: member accounts,
> earn/spend operations, live balances, role-based access, and safe CSV batch ingestion.
> Written in **Go**, persisted in **SQLite**.

<p align="left">
  <img alt="status" src="https://img.shields.io/badge/status-step%203%3A%20build%20(next)-blue">
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
  [`docs/slices/`](docs/slices/), and **6 GitHub issues opened** (#1–#6). Build starts at
  **[S0 / #1](https://github.com/ossewawiel/gowallet/issues/1)** — the walking skeleton.

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

> ⚠️ **Not runnable yet** — the service code arrives at slice **S0** (Step 3). This section fills in
> with real commands the moment there's something to run.

```bash
# Coming at slice S0:
#   go run ./cmd/gowallet          # start the API
#   open http://localhost:8080/swagger
```

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
_(application code — `cmd/`, `internal/`, `api/`, `test/` — lands in Step 3.)_
