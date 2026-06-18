# CLAUDE.md — gowallet house rules 🪙

> Read this first, every session. It's the index + the non-negotiables. Details live in `docs/`.

gowallet is a **loyalty points wallet**: accounts earn/spend points, balances stay correct under
duplicates and concurrency, with `member`/`admin` access and CSV batch ingestion. Go + SQLite.
The brief is `docs/specifications.pdf` — **always the final word**.

---

## 🗣️ House voice & output style (applies to EVERY reply, going forward)

This one's a standing order, not a suggestion:

- **Casual banter, low lingo.** Talk like a helpful teammate, not a whitepaper. Explain the *why*
  in plain language. The user is new to Go — define a term the first time it shows up, briefly.
- **Make it visual.** Lean on `tables`, **frames/boxes**, bullet points, and icons (✅ ⚠️ 🚧 🔐 🧪
  ➡️ 📦 🗄️). A wall of prose is a smell.
- **Show the workflow.** Indicate progress, what's done, what's next. Checklists and status frames
  over paragraphs.
- **Concrete over hand-wavy.** Real commands, real file paths, real numbers.

---

## 🧱 The stack (locked — don't relitigate without a good reason)

| Layer | Choice | Notes |
|-------|--------|-------|
| Language | **Go 1.26.x** | stdlib-first |
| Routing | **stdlib `net/http`** (1.22 method routing) **+ `chi`** | `chi` for middleware + sub-routers |
| API contract | **Spec-first OpenAPI 3** → **`oapi-codegen`** (strict-server) | `api/openapi.yaml` is the source of truth |
| Request validation | **`kin-openapi`** middleware | validates requests against the spec |
| Database | **SQLite** via **`modernc.org/sqlite`** (pure Go) | swap-ready for Postgres later |
| DB access | **`sqlc`** (compile SQL → typed Go) | no ORM, no runtime reflection |
| Migrations | **`goose`**, **timestamped** versions | timestamped avoids collisions across parallel branches |
| Auth | **JWT HS256** via `golang-jwt/jwt` | `WithValidMethods(["HS256"])` enforced; `role` + `sub` claims |
| Contract testing | **Schemathesis** (property + stateful) | drives e2e from the spec |
| Unit/integration | **Go `testing` + `testify`**, `go test -race` | strict TDD |

➡️ Full reasoning + project layout: **`docs/ARCHITECTURE.md`**.

---

## 🥇 Golden rules

1. **TDD, always. Red → Green → Refactor.** The spec violation is the first red; Go tests sit
   below it. No production code without a failing test first. See `docs/DEVELOPMENT_FLOW.md`.
2. **Spec-first.** Change `api/openapi.yaml` → regenerate → make it pass. The spec leads, code follows.
3. **Wire-crossing is structurally impossible.** The ONLY things shared across requests are the
   `*sql.DB` pool and config. Everything request-specific (user, role, deadline) rides in
   `r.Context()`. Prove it with parallel-submission `-race` tests.
4. **Money rules are SQL-level.** `UNIQUE(ref)` for idempotency; balance checks inside the same
   transaction as the write; never let a spend go negative.
5. **Vertical slices.** A slice = one full REST cycle (handler → service → store → migration →
   spec → tests), shippable on its own. Not one function at a time. See `docs/SLICES.md`.
6. **Layering is one-directional:** `api → domain → store`. `domain` imports nothing upward.

---

## 🔁 The prompt log — keep it alive (standing order)

After every meaningful exchange or chunk of work, **append an entry to `docs/PROMPT_LOG.md`**
(curated: 🧑 asked · 🤖 did · ✅ accepted / ✏️ edited · 💡 why). This is a hard requirement from
the brief — it's how the AI workflow gets graded. Prefer the **`doc-updater`** subagent for this
so it doesn't clog the main context. Keep `SOLUTION.md` in sync when decisions change.

---

## 🧰 Where the tooling lives

| Type | What it's for | Where |
|------|---------------|-------|
| **Commands** (you type them) | Kick off a process; pull the right skills + docs; enforce the flow | `.claude/commands/` |
| **Skills** (auto-pulled) | House rules: architecture, REST, TDD | `.claude/skills/` |
| **Subagents** (isolated context) | `doc-updater`, `tdd-runner` | `.claude/agents/` |
| **Docs** (the knowledge) | Architecture, REST, flow, acceptance, slices | `docs/` |

**Main commands:** `/design-slice` (design + open the GitHub issue) · `/build-slice <issue#>`
(TDD-build it) · `/quality-gate` (compile + vet + lint + `-race` + Schemathesis) · `/log-progress`.

---

## 🚦 Quality gate (must be green before "done")

`gofmt` ✓ · `go vet` ✓ · `golangci-lint` ✓ · `go build ./...` ✓ · `go test -race ./...` ✓ ·
Schemathesis ✓ · `docs/ACCEPTANCE.md` invariants for the slice ✓ · `docs/PROMPT_LOG.md` updated ✓.
