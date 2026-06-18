# 🏗️ Architecture

How gowallet is put together, and the rules that keep it clean. If CLAUDE.md is the "what,"
this is the "where and why."

---

## 🎯 The shape: thin layers, one-way dependencies

We use a light **hexagonal / clean-ish** layout. The point isn't dogma — it's that **business
rules don't know about HTTP or SQL**, so we can swap SQLite → Postgres later by changing one layer.

```
            HTTP request
                 │
                 ▼
   ┌────────────────────────────────┐
   │ internal/httpapi  (transport)  │  ← oapi-codegen handlers, JWT middleware, router, JSON in/out
   └───────────────┬────────────────┘
                   │ calls services (interfaces defined in wallet)
                   ▼
   ┌────────────────────────────────┐
   │ internal/wallet   (THE CORE)   │  ← Account, Transaction, balance + idempotency rules,
   │  imports nothing internal      │     services, repository interfaces, sentinel errors
   └───────────────▲────────────────┘
                   │ implements wallet's repository interfaces
   ┌───────────────┴────────────────┐
   │ internal/sqlitestore (SQLite)  │  ← sqlc queries, goose migrations, DB open + PRAGMAs
   └────────────────────────────────┘
```

**Dependency rule:** `httpapi → wallet ← sqlitestore`. Both edges point **at** `wallet`; `wallet`
imports neither.
- `wallet` defines interfaces like `AccountRepository`; `sqlitestore` implements them.
- `httpapi` depends on `wallet` (calls its services), never on `sqlitestore` directly.
- `cmd/gowallet/main.go` is the only place that wires all three together.
- Result: `wallet` is pure Go you can unit-test with zero database.

> 🧩 **Only three internal packages.** Auth (JWT) isn't its own package — it's `httpapi` middleware
> plus a `/token` handler. CSV batch ingestion is an `httpapi` handler that calls `wallet` services.
> The audit writer is a `wallet` service persisted by `sqlitestore`. Keep it at three.

---

## 📂 Project layout

```
gowallet/
├── cmd/gowallet/main.go         # entrypoint: wire config → sqlitestore → wallet → httpapi → serve
├── api/
│   └── openapi.yaml             # 📜 SOURCE OF TRUTH for the contract (spec-first)
├── internal/
│   ├── httpapi/                 # TRANSPORT: handlers, middleware (jwt, request-id, recover), router,
│   │   │                        #            the /token endpoint, the CSV batch handler
│   │   └── gen/                 # oapi-codegen OUTPUT (do not hand-edit)
│   ├── wallet/                  # THE CORE: Account, Transaction, balance/idempotency rules,
│   │                            #           services, repo interfaces, sentinel errors, audit writer
│   └── sqlitestore/             # PERSISTENCE: implements wallet's repo interfaces
│       ├── gen/                 # sqlc OUTPUT (do not hand-edit)
│       ├── queries/             # *.sql  → sqlc reads these
│       ├── migrations/          # *.sql  → goose, TIMESTAMPED filenames
│       └── sqlite.go            # DB open + PRAGMAs (WAL, busy_timeout)
├── test/
│   ├── acceptance/              # Go -race tests proving docs/ACCEPTANCE.md invariants
│   └── schemathesis/            # schemathesis run config
├── docs/                        # you are here
├── .claude/                     # skills, commands, agents
└── .github/                     # issue templates, CI
```

> 🧠 **Why `internal/`?** Go won't let anything outside this module import `internal/...`. It's a
> compiler-enforced "private" — keeps our guts from becoming someone else's dependency.

---

## 🗄️ The database rules (where correctness lives)

SQLite is single-writer, multi-reader. We lean into that:

- **PRAGMAs on every connection:** `journal_mode=WAL`, `busy_timeout=5000`, `synchronous=NORMAL`,
  `foreign_keys=ON`. WAL lets readers run while the one writer works; `busy_timeout` waits instead
  of erroring under contention.
- **One writer.** Set `db.SetMaxOpenConns(1)` for the write path (or a dedicated writer handle) so
  we never trip `SQLITE_BUSY`. Reads can use a wider pool.
- **Invariants are constraints, not vibes:**
  - `transactions.ref` has a **`UNIQUE`** index → the same `ref` physically can't be stored twice.
  - Balance is changed **inside the same transaction** that records the spend; the no-negative
    check and the write are atomic.
- **Durability:** on-disk file (`*.db`), survives restarts. (The file itself is git-ignored.)

---

## 🔌 The wire-crossing guarantee (multi-user safety)

Go's `net/http` runs **each request in its own goroutine** with its **own `*http.Request` and
`r.Context()`**. So there's no shared per-request slot to collide in — *unless we create one*.

**The rule:** the only things shared across requests are the **`*sql.DB` pool** and **config**.
Everything request-specific (the authenticated account, role, deadline) lives in `r.Context()`.

Follow that and cross-wiring is structurally impossible. We still prove it (trust, but verify):
`go test -race` + parallel-submission tests where N users hit their own accounts and we assert
nobody sees anyone else's data. See `docs/ACCEPTANCE.md`.

---

## 🔐 Auth in one breath

JWT, **HS256**, signed with a server secret. Token carries `sub` (account_id) + `role`
(`member`/`admin`). Middleware verifies with `golang-jwt` **pinning the method**
(`WithValidMethods(["HS256"])`) to dodge the `alg:none` / algorithm-confusion attacks, then drops
the identity into the request context. Handlers read identity from context — never from the URL.

---

## 🔄 Swap-ability (the "Postgres later" promise)

Because `wallet` talks to `sqlitestore` through interfaces and `sqlitestore` is the only place that
knows SQL dialects, moving to Postgres is: new migrations, point sqlc at the `postgres` engine, swap
the driver in `sqlite.go`. The handlers and rules don't change. That's the whole reason for the layering.
