# 🪙 gowallet — Loyalty Points Wallet

> A small, production-minded backend for a **loyalty points wallet**: member accounts,
> earn/spend operations, live balances, role-based access, and safe CSV batch ingestion.
> Written in **Go**, persisted in **SQLite**.

<p align="left">
  <img alt="status" src="https://img.shields.io/badge/status-step%201%3A%20plumbing-yellow">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26.x-00ADD8">
  <img alt="db" src="https://img.shields.io/badge/SQLite-pure--Go%20driver-003B57">
</p>

---

## 🧭 Where this project is right now

This repo is built in deliberate stages so the history reads like a story, not a data-dump:

| Stage | What it covers | State |
|------:|----------------|:-----:|
| **1. Plumbing** | Toolchain (Go, gopls), GitHub repo, prompt-recording timeline | 🚧 in progress |
| **2. Dev system** | Project layout, TDD harness, OpenAPI/Swagger, Docker, CI | ⏳ next |
| **3. Design & build** | Data model, endpoints, auth, batch ingestion, concurrency safety | ⏳ later |

> 🔎 The full blow-by-blow of how this was built — every prompt, decision and trade-off —
> lives in [`docs/PROMPT_LOG.md`](docs/PROMPT_LOG.md) and is summarised in [`SOLUTION.md`](SOLUTION.md).

---

## 🧱 Tech stack (decided so far)

| Concern | Choice | Why (short version) |
|---------|--------|---------------------|
| Language | **Go 1.26.x** | Assignment requirement; tiny, fast, great at safe concurrency |
| Database | **SQLite** via `modernc.org/sqlite` | **Pure Go** — no C compiler, so anyone can `git clone && go run` |
| Persistence | Single `.db` file, **WAL mode** | Durable across restarts; readers don't block the writer |
| API docs | **OpenAPI 3 + Swagger UI** | Self-documenting, click-to-test endpoints |
| Testing | **Go `testing` (strict TDD)** + Playwright | Red → green → refactor, always |
| Packaging | **Docker** | Run identically on any machine, no local Go needed |
| Auth | _TBD (Step 3)_ — roles: `member`, `admin` | Members manage their own wallet; admins see all |

> 💡 For longer-term production we'd swap SQLite for **PostgreSQL/MariaDB** — the data
> access layer is being designed so that swap is a driver change, not a rewrite.

---

## 🚀 Quickstart

> ⚠️ **Not runnable yet** — the service code arrives in Step 3. This section will fill in with
> real commands (`go run ./cmd/gowallet`, Docker, Swagger URL) as soon as there's something to run.

```bash
# Coming in Step 3:
#   go run ./cmd/gowallet        # start the API
#   open http://localhost:8080/swagger
```

---

## 📂 Repository layout

```
gowallet/
├── README.md           ← you are here
├── SOLUTION.md         ← design, trade-offs, and the AI workflow write-up
├── docs/
│   ├── specifications.pdf   ← the original assignment brief
│   └── PROMPT_LOG.md        ← chronological timeline of prompts + decisions
└── .vscode/            ← shared editor config (Go extension + gopls)
```

_(application code directories land in Steps 2–3.)_
