# CLAUDE.md — gowallet house rules 🪙

> Read this first, every session. It's the index + the non-negotiables. Details live in `docs/`.

gowallet is a **loyalty points wallet**: accounts earn/spend points, balances stay correct under
duplicates and concurrency, with `member`/`admin` access and CSV batch ingestion. Go + SQLite.
The brief is `docs/specifications.pdf` — **always the final word**.

---

## 🗣️ House voice & output style (applies to EVERYTHING written, going forward)

This one's a standing order, not a suggestion. It covers **both** chat replies **and every document
we write** — `README.md`, `SOLUTION.md`, everything in `docs/`, GitHub issue & PR bodies, even
comments where it helps. If it's prose a human reads, it follows this voice.

- **Casual banter, low lingo.** Talk like a helpful teammate, not a whitepaper. Explain the *why*
  in plain language. The user is new to Go — define a term the first time it shows up, briefly.
- **Make it visual.** Lean on `tables`, **frames/boxes**, bullet points, and icons (✅ ⚠️ 🚧 🔐 🧪
  ➡️ 📦 🗄️). A wall of prose is a smell.
- **Show the workflow.** Indicate progress, what's done, what's next. Checklists and status frames
  over paragraphs.
- **Concrete over hand-wavy.** Real commands, real file paths, real numbers.

> 📌 **Scope note:** the only places we *don't* get cute — keep these conventional/clean: commit
> message subject lines, generated code, and `api/openapi.yaml` (it's a machine contract). Their
> surrounding prose (PR descriptions, doc explanations) still uses the house voice.

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
6. **Layering:** `httpapi → wallet ← sqlitestore` — both edges point at `wallet`, which imports
   neither. **Three** internal packages, no more (auth = `httpapi` middleware; CSV = `httpapi` handler).

---

## 🔁 The prompt log — keep it alive & granular (standing order)

After every meaningful exchange or decision, **append to `docs/PROMPT_LOG.md`**. This is a hard
requirement from the brief — it's how the AI workflow gets graded — so err toward **more detail,
finer grain**:

- **One entry per exchange or decision**, not per whole "step." If a single message holds 3
  decisions, log 3 — don't collapse them into one vague line.
- Each entry captures: ⏱️ when · 🧑 **Asked** (the real request, tightly paraphrased) · 🔎 **Explored**
  (options weighed + roads not taken) · 🤖 **Did** (files, commands, installs) · ✅ **Accepted** /
  ✏️ **Edited** / ❌ **Rejected** (each decision + its reason) · 💡 **Why** · 📚 **Sources** ·
  🔗 **Artifacts** (commit SHAs, file paths, issue/PR #s).
- Still **curated, not a transcript** — capture the signal and the *why*, skip the noise.
- Prefer the **`doc-updater`** subagent so it doesn't clog the main context. Keep `SOLUTION.md` in
  sync when a decision changes.

---

## 📈 README tracks progression (standing order)

`README.md` is the **front door** — a reviewer should see current state at a glance without digging
through the prompt log. Keep its **Progress** section accurate and **update it on every push** (what
step/slice just landed, what's next). The **`doc-updater`** subagent owns this alongside the prompt log.

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
Schemathesis ✓ · `docs/ACCEPTANCE.md` invariants for the slice ✓ · `docs/PROMPT_LOG.md` updated ✓ ·
`README.md` progression updated ✓.

---

## 🪟 Running the gates on Windows (standing reference — don't rediscover this)

The toolchain isn't all on the default **bash** PATH. Use the **PowerShell** tool for builds/tests,
and reach for these exact locations:

| Tool | Where | Notes |
|------|-------|-------|
| `go` | on PowerShell PATH (`go1.26.x`) | **not** on the bash PATH — use PowerShell |
| `oapi-codegen` | `C:\Users\User-PC\go\bin\oapi-codegen.exe` | `oapi-codegen -config internal/httpapi/gen/config.yaml api/openapi.yaml` |
| `sqlc` | `C:\Users\User-PC\go\bin\sqlc.exe` | `sqlc generate` (only when a query/schema changed) |
| `golangci-lint` | `C:\Users\User-PC\go\bin\golangci-lint.exe` | |
| `schemathesis` | `C:\ocsl\python313\Scripts\schemathesis.exe` | needs `PYTHONUTF8=1` on Windows |
| **MinGW gcc** (for `-race`) | `C:\Users\User-PC\scoop\apps\mingw\current\bin` | `gcc 16.1.0`; Go rejects MSVC clang |

### 🏁 `go test -race` needs cgo + MinGW (NOT the default env)
The pure-Go SQLite driver means `CGO_ENABLED=0` by default, but `-race` **requires** cgo. Prefix
every race run with:
```powershell
$env:Path = "C:\Users\User-PC\scoop\apps\mingw\current\bin;$env:Path"; $env:CGO_ENABLED = "1"
go test -race ./...
```

### 🧪 Schemathesis MUST run with a Bearer token (the standardised recipe)
Almost every route is `bearerAuth`-protected, so a token-less `schemathesis run` drowns in false
401s. **Always** boot the server with a known secret, mint an **admin** token via `/token`, and pass
it as a header. Admin can act on any account, so it exercises every operation.

```powershell
$env:PYTHONUTF8 = "1"
$env:GOWALLET_JWT_SECRET = "schemathesis-secret"   # any non-empty value; boot fails without it
# 1) boot the server (background), wait for /healthz
# 2) mint an admin token:
$body = @{ account_id = "test-admin"; role = "admin" } | ConvertTo-Json
$tok  = (Invoke-RestMethod "http://localhost:8080/token" -Method Post -Body $body -ContentType "application/json").token
# 3) run against the SERVED spec, with the token on every request:
schemathesis run "http://localhost:8080/openapi.yaml" -u "http://localhost:8080" `
  -H "Authorization: Bearer $tok" --exclude-checks negative_data_rejection
```

> ⚠️ **Known, accepted non-issue:** `POST /token` is reported by Schemathesis v4's
> `positive/negative_data_rejection` checks because `role` is validated **semantically** (unknown
> role → `422`), not as a schema enum (deliberate — see the spec comment on `TokenRequest.role`).
> That single `/token` finding is **expected** and is **not** a gate failure. Every other operation
> must pass clean. The `--exclude-checks negative_data_rejection` flag trims the matching noise.
