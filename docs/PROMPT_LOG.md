# 🗂️ Prompt & Decision Timeline

A running, plain-language log of how gowallet was built — the prompts asked, what the AI did,
what was accepted or edited, and **why**. Newest entries at the bottom. This is the "director's
commentary" for [`SOLUTION.md`](../SOLUTION.md).

**Legend:** 🧑 asked · 🔎 explored/options · 🤖 did · ✅ accepted · ✏️ edited/steered · ❌ rejected ·
💡 why · 📚 source · 🔗 artifacts (commits/paths/issues)

> 📏 **Granularity:** one entry per meaningful exchange or decision — finer is better. Don't fold
> multiple decisions into a single line. Curated, not a transcript.

---

## Step 1 — Plumbing

### ⏱️ 2026-06-18 · Entry 01 — Framing the assignment

- 🧑 **Asked:** Build gowallet (Go REST API + SQLite) in three stages — plumbing, dev system,
  then design & build. I'm new to Go; keep explanations brief but proper. Set up Go, SQLite, and
  a GitHub repo, and record every prompt/answer as a timeline. Tone: relaxed, visual, descriptive.
- 🤖 **Did:** Ran a deep-interrogation pass instead of jumping to code. Read the spec
  (`docs/specifications.pdf` — an OfferZen/Sanlam SFTX Senior Engineer take-home: loyalty points
  wallet, ~4h, README + SOLUTION.md + Loom demo). Probed the machine: only `git` present — no Go,
  `gh`, C compiler, or `sqlite3` CLI. Pulled primary sources and posed the decision-flipping
  questions.
- 💡 **Why:** A wallet is correctness-critical; choosing the driver, repo model and test posture
  up front avoids expensive rework. 📚 [go.dev/dl](https://go.dev/dl/),
  [sqlite.org/wal](https://www.sqlite.org/wal.html).

### ⏱️ 2026-06-18 · Entry 02 — Decisions locked

Answers to the interrogation, which set the project's direction:

| Question | Decision | ✅/✏️ |
|----------|----------|:----:|
| SQLite driver | **`modernc.org/sqlite`** (pure Go, no C compiler) — portability | ✅ |
| SQLite CLI? | Not needed; service persists to a `.db` file | ✏️ |
| Repo visibility | **Public** | ✏️ |
| Repo tooling | Install **`gh` CLI** | ✅ |
| Prompt log style | Curated "asked / accepted / edited / why" (this file) | ✅ |
| Go install | Via **winget** | ✅ |
| Editor | VS Code present; primary dev via the coding agent | — |
| Scope | **Over-deliver:** OpenAPI + Swagger UI, Playwright tests, strict TDD, Docker | ✏️ |

- 💡 **Why pure-Go driver:** with no C compiler installed, `mattn/go-sqlite3` would add a MinGW
  detour and break clone-and-run; `modernc.org/sqlite` trades some write speed (irrelevant here)
  for zero-toolchain portability. 📚 [driver trade-off](https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html).

### ⏱️ 2026-06-18 · Entry 03 — Plumbing execution kicked off

- 🧑 **Asked:** Go for it — set up the environment and the repo.
- 🤖 **Did:** Installed Go + `gh` via winget (background); added a task for **gopls** (the official
  Go LSP server) so the agent and VS Code get fast code navigation; scaffolded the repo
  (`README.md`, `SOLUTION.md`, this log, `.gitignore`, shared `.vscode/` config).
- ✏️ **Steered:** Added gopls + LSP wiring on top of the base ask, after I flagged needing a
  language server for the agent.
- 💡 **Why:** gopls is installed separately from the compiler (`go install
  golang.org/x/tools/gopls@latest`) and lives in `%USERPROFILE%\go\bin`, which isn't on PATH by
  default — handling it now prevents "command not found" later.
  📚 [gopls](https://pkg.go.dev/golang.org/x/tools/gopls).

### ⏱️ 2026-06-18 · Entry 04 — Plumbing complete ✅

- 🤖 **Did:** Installed the toolchain via winget and verified everything end-to-end:

  | Tool | Version | Notes |
  |------|---------|-------|
  | Go | `go1.26.4 windows/amd64` | latest stable |
  | GitHub CLI | `gh 2.94.0` | was already installed → upgraded |
  | gopls (Go LSP) | `v0.22.0` | on PATH at `C:\Users\User-PC\go\bin` |

- ✅ **Accepted:** `gh` turned out to be **already authenticated** (account `ossewawiel`, SSH,
  `repo` scope) — so no interactive login was needed.
- 🤖 **Did:** Created the **public** repo and pushed the first commit:
  **https://github.com/ossewawiel/gowallet** (remote `origin` over SSH; `main` tracks `origin/main`).
- 💡 **Why this order:** install → refresh PATH in-session → verify each tool → only then create the
  repo, so a broken tool surfaces immediately instead of mid-push.

**Step 1 outcome:** environment ready, repo live, recording in place. ➡️ Next: **Step 2 — dev system**
(project layout, TDD harness, OpenAPI/Swagger, Docker, CI).

## Step 2 — Dev system

### ⏱️ 2026-06-18 · Entry 05 — Stack locked + development system built ✅

- 🧑 **Asked:** What Go libs/frameworks to use, then set up CLAUDE.md, skills, subagents, GitHub
  issue templates, and an issue-driven, vertical-slice, TDD workflow. Tone: casual banter, low
  lingo, visual output — *going forward, always*.
- 🤖 **Did:** Ran a two-round interrogation with primary sources, then locked the stack and built
  the system.

  | Decision | Pick | Killed |
  |----------|------|--------|
  | Routing | stdlib `net/http` (1.22) + `chi` | gin / echo / fiber |
  | API contract | spec-first `oapi-codegen` + `kin-openapi` | code-first / hand-rolled |
  | DB access | `sqlc` + `goose` (timestamped) | GORM / sqlx |
  | Auth | JWT **HS256** (`golang-jwt`, method pinned) | opaque / PASETO / RS256 |
  | Testing | Schemathesis (contract) + Go `-race` (invariants) | Playwright on contract path |

- ✏️ **Steered:** Source of truth = **two layers** (OpenAPI spec + `docs/ACCEPTANCE.md` invariants
  registry). Concurrency proven by **parallel-submission `-race` tests** (no load tool). **3 parallel
  streams**, **auth midstream**. JWT confirmed over PASETO.
- 💡 **Why JWT HS256, not RS256:** gowallet is a single service that signs *and* verifies — RS256's
  public-key split buys nothing here; HS256 + `WithValidMethods` is simpler and dodges alg-confusion.
- 🤖 **Built:** `CLAUDE.md` (golden rules + house voice + prompt-log rule), 5 `docs/` guides
  (architecture, REST, dev-flow, acceptance, slices), 3 skills (`go-architecture`,
  `rest-api-standards`, `tdd-workflow`), 2 subagents (`doc-updater`, `tdd-runner`), 4 commands
  (`design-slice`, `build-slice`, `quality-gate`, `log-progress`), and GitHub slice issue templates.
- 💡 **The model:** a command pulls the right skills → which read the right docs → enforcing process
  flow + feedback + progression. Design happens in the main session → a fully-specced GitHub issue →
  a fresh session (`tdd-runner`) builds it with no re-design.

**Step 2 outcome:** the factory is built. ➡️ Next: **Step 3 — execution** (start at slice **S0**,
the walking skeleton, via `/design-slice`).

### ⏱️ 2026-06-18 · Entry 06 — Prompt-log rule: go granular

- 🧑 **Asked:** Make prompt-log entries more granular; update the rules (not just this one entry).
- 🔎 **Explored:** Where the rule lives — `CLAUDE.md` (standing order), `.claude/agents/doc-updater.md`
  (the writer's spec), and this file's legend. Weighed rewriting past entries vs rules-only →
  chose rules-only (can backfill on request).
- ✅ **Accepted:** "One entry per exchange/decision" granularity + a richer entry skeleton.
- ✏️ **Edited:** Added icons to the format — 🔎 explored, ❌ rejected, 🔗 artifacts.
- 🤖 **Did:** Updated `CLAUDE.md`, `.claude/agents/doc-updater.md`, and this file's legend. This entry
  is the first written in the new style.
- 💡 **Why:** The brief grades the AI workflow — finer grain shows the real reasoning and the roads
  not taken, not just milestones.
- 🔗 **Artifacts:** CLAUDE.md · docs/PROMPT_LOG.md · .claude/agents/doc-updater.md

### ⏱️ 2026-06-18 · Entry 07 — README tracks progression, updated every push

- 🧑 **Asked:** README wasn't reflecting the latest progress; it should track progression and be
  updated on every push.
- 🤖 **Did:** Refreshed `README.md` (Step 1 ✅ + Step 2 ✅ + Step 3 next, new **Progress** section,
  locked tech-stack table, updated layout). Added a standing order in `CLAUDE.md` + the quality gate,
  and gave `doc-updater` ownership of README progression.
- ✅ **Accepted:** "Update README every push" as a definition-of-done item.
- 💡 **Why:** README is the front door — a reviewer should see current state at a glance without
  digging through the prompt log.
- 🔗 **Artifacts:** README.md · CLAUDE.md · .claude/agents/doc-updater.md

## Step 3 — Execution

### ⏱️ 2026-06-18 · Entry 08 — Slice catalogue finalized (8 → 6)

- 🧑 **Asked:** Split Step 3 into slices, generate a kickoff prompt per slice for fresh sessions, and
  lay out an order that hits parallelism ASAP.
- 🔎 **Explored:** Re-read the brief and pressure-tested the Step-2 `SLICES.md` against primary
  sources — Cockburn's walking skeleton, the IETF idempotency-key draft, Russ Cox on Go layout.
- ✅ **Accepted:** keep S0 a pure walking skeleton · bake concurrency into S1/S2 (no separate
  hardening slice) · keep audit its own slice for parallelism · `ref` IS the idempotency key (body,
  `UNIQUE`).
- ✏️ **Edited:** collapsed accounts+earn into one slice (S1) · flattened the 5-package `internal/` to
  **3 packages** (`httpapi → wallet ← sqlitestore`) · auth (S3) runs parallel from right after S0.
- ❌ **Rejected:** layering an IETF `Idempotency-Key` header on top of `ref` (overkill for the brief);
  the 5-package layout (Go team favours simpler).
- 💡 **Why:** fewer, honest slices; only S0 is truly serial; a reviewer reads a 3-package tree in minutes.
- 📚 [Cockburn](https://yoshi389111.github.io/kinokobooks/soft_en/Start_with_a_Walking_Skeleton.htm) ·
  [IETF idempotency draft-07](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-07) ·
  [Russ Cox on layout](https://github.com/golang-standards/project-layout/issues/117)
- 🔗 **Artifacts:** docs/SLICES.md · docs/ARCHITECTURE.md · docs/ACCEPTANCE.md

### ⏱️ 2026-06-18 · Entry 09 — Slice prompts + GitHub issues created

- 🤖 **Did:** Wrote 6 medium kickoff prompts (`docs/slices/S0–S5.md`) + a run-order guide
  (`docs/slices/README.md`); created the `slice` + `stream-a/b/c` labels and **opened 6 issues
  (#1–#6)** straight from the prompt files; remapped the ACCEPTANCE invariants to the new slice IDs;
  taught `/design-slice` to enrich an existing issue instead of opening a duplicate.
- ✅ **Accepted:** one file per slice + a master order doc; medium-rich prompts (intent + invariants +
  traps + deps), not full pre-designs.
- 💡 **Why:** each issue is a fresh-session entry point — paste prompt → `/design-slice` fleshes the
  issue → `/build-slice` ships it. Build starts at **S0 (#1)**.
- 🔗 **Artifacts:** docs/slices/*.md · GitHub issues #1–#6 · .claude/commands/design-slice.md

### ⏱️ 2026-06-18 · Entry 10 — S0 designed (walking skeleton, design-only)

- 🧑 **Asked:** Run `/design-slice` for **S0** — the walking skeleton (architecture tracer bullet).
  Design-only; no production code yet.
- 🔎 **Explored:**
  - **INV-n rows for S0?** → ❌ No. `docs/ACCEPTANCE.md` is scoped to *business + concurrency*
    invariants; S0 has neither (it's the structural tracer). The S0 slice doc already says
    "Invariants: none." So no rows added.
  - **Does `/healthz` belong in the OpenAPI contract?** → ✅ Yes, so Schemathesis can fuzz it.
    `/openapi.yaml` and `/swagger` stay **infra routes**, not spec paths.
  - **Empty initial goose migration?** → ✅ Yes, intentionally empty (`SELECT 1` up/down). goose still
    creates `goose_db_version` on startup, which the acceptance test asserts — proves the runner works
    **without inventing tables S1 owns**.
  - **Seed the shared Error envelope now?** → ✅ Yes, even though `/healthz` doesn't need it — so every
    later slice reuses **one** error shape (per REST guidelines).
  - **Health seam design:** `wallet` defines a `Pinger` interface + `HealthService`; `sqlitestore`
    implements `Pinger` via `*sql.DB.PingContext`; `httpapi` calls `wallet`. Proves the layering
    direction `httpapi → wallet ← sqlitestore` through a trivial path.
- 🤖 **Did:** Read the S0 slice doc + `ARCHITECTURE.md`, `ACCEPTANCE.md`, `SLICES.md`,
  `REST_API_GUIDELINES.md`, `DEVELOPMENT_FLOW.md`, and the slice issue template. Confirmed the repo has
  **no `go.mod`/`api/` yet** (S0 creates them). **Enriched GitHub issue #1** (was just the kickoff
  prompt) with the full build spec: OpenAPI fragment (`/healthz` + Health + Error schemas), timestamped
  goose migration (`20260618000000_init.sql`), domain/health seam, a **7-test red list**, acceptance
  criteria, and build order. Issue updated via `gh issue edit 1 --body-file`.
- ✅ **Accepted:** the full S0 design above.
- ❌ **Rejected:** adding `INV-n` rows for S0 — no business/concurrency invariant exists to prove.
- 💡 **Why:** S0 is a **kept walking skeleton** (Cockburn), not a throwaway spike — nailing the
  3-package wiring + DB PRAGMAs + migration runner *once* means every later slice slots in. Tests still
  ship (health path, PRAGMAs, migration runner) so production habits hold from line one.
- 📚 `docs/ARCHITECTURE.md` (3-package layout, PRAGMAs, wire-crossing rule) · `docs/DEVELOPMENT_FLOW.md`
  (TDD loop) · `docs/REST_API_GUIDELINES.md` (error envelope, status codes) · `docs/ACCEPTANCE.md`
  (invariant scope).
- 🔗 **Artifacts:** GitHub issue #1 (enriched) — https://github.com/ossewawiel/gowallet/issues/1 ·
  no code commits (design-only).

### ⏱️ 2026-06-18 · Entry 11 — S0 built (walking skeleton, red→green→quality-gate) ✅

- 🧑 **Asked:** Run `/build-slice` for **S0** (issue #1) — strict spec-first TDD. First slice, so
  scaffold the repo from scratch.
- 🔎 **Explored / decisions in-build:**
  - **Schemathesis caught a real contract nit:** `GET /healthz` with a bad method returned `405`
    but **no `Allow` header** — RFC 9110 wants one. Fixed with a `MethodNotAllowed` handler that
    sets `Allow`, plus an **8th test** `TestHealthz_405_HasAllowHeader` — wrote the failing test
    *first*, then re-greened. TDD held even for the contract fix. ✅
  - **Hand-written JSON bytes for `/healthz`** (not `json.Marshal` of a map) — guarantees key order
    matches the spec example byte-for-byte. ✅
  - **PRAGMAs ride on the SQLite DSN**; write path pins `SetMaxOpenConns(1)`. ✅
  - **Layering verified with `go list -deps`:** `httpapi` imports `wallet` + `gen`, **never**
    `sqlitestore`; `wallet` imports nothing internal. The arrow `httpapi → wallet ← sqlitestore`
    holds structurally, not just by convention. ✅
- 🤖 **Did:** Branched `slice/s0-skeleton` off `main`. Scaffolded the repo:
  - `go.mod` (module `github.com/ossewawiel/gowallet`, Go 1.26.4)
  - `api/openapi.yaml` (`/healthz` + shared `Error` envelope)
  - `oapi-codegen` strict chi-server output → `internal/httpapi/gen/`
  - `internal/httpapi` (router, middleware, `health.go`, `errors.go` envelope + 404/405,
    `infra.go` for `/openapi.yaml` + `/swagger`)
  - `internal/wallet` (`Pinger`, `Health`, `HealthService`)
  - `internal/sqlitestore` (`Open` w/ PRAGMAs, `Migrate` via goose, `Ping`) + timestamped empty
    migration `20260618000000_init.sql`
  - `cmd/gowallet/main.go` wiring it all together
  - **8 tests** (7 from the issue + the 405 one). Loop: red → green → refactor → full quality gate.
- ✅ **Accepted:** every gate green — `gofmt` clean · `go vet` 0 · `golangci-lint` 0 issues ·
  `go build` ok · `go test -race` ok · Schemathesis **9 cases, no issues**. All S0 acceptance
  criteria met.
- 💡 **Why:** S0 is a **kept** walking skeleton, not a spike — getting the 3-package wiring +
  PRAGMAs + migration runner right *once* means every later slice slots straight in.
- 🛠️ **Tooling notes worth keeping:**
  - `-race` on Windows needs a **real gcc** — installed MinGW **gcc 16.1.0** via `scoop install
    mingw` (Go rejects MSVC-targeted clang's `-mthreads`). Future race runs: put
    `C:\Users\User-PC\scoop\apps\mingw\current\bin` on PATH + `CGO_ENABLED=1`.
  - Schemathesis (Python) needs `PYTHONUTF8=1` on cp1252 consoles.
- 📚 GitHub issue #1 (the build spec) · `docs/ARCHITECTURE.md` · `docs/DEVELOPMENT_FLOW.md` ·
  `docs/REST_API_GUIDELINES.md` · RFC 9110 (`Allow` header on 405).
- 🔗 **Artifacts:** branch `slice/s0-skeleton` (not yet pushed/committed at time of writing) ·
  files created — `go.mod`, `go.sum`, `api/openapi.yaml`, `internal/httpapi/*` (+`gen`),
  `internal/wallet/health.go`, `internal/sqlitestore/*` (+migration), `cmd/gowallet/main.go`,
  tests in `internal/*/*_test.go` + `test/acceptance/healthz_test.go`. Closes #1 once the PR merges.

### ⏱️ 2026-06-18 · Entry 12 — S1 designed (Accounts + Earn + Balance, design-only)

- 🧑 **Asked:** Run `/design-slice` for **S1** — Accounts + Earn + Balance. Design-only; enrich the
  existing GitHub issue **#2**, no production code.
- 🔎 **Explored / decisions weighed (and roads not taken):**
  - **`account_id` is client-supplied** (brief's example `"member-123"`), not server-generated → a
    duplicate create is a **409 `account_exists`**. (Not auto-incrementing IDs the server hands back.)
  - **Idempotent replay** of a known `ref` returns **200** with the *stored* txn (first-write-wins) —
    **not 201, not 409**. Replay with a *different* payload still returns the stored txn; `ref` is the
    idempotency key. ❌ Rejected: 409-on-replay; ❌ rejected: comparing payloads to detect conflicts.
  - **`kind` enum is `[earn]`-only** in the S1 spec, so `kin-openapi` bounces a `spend` at the edge
    (**400**). The DB `CHECK` already allows `('earn','spend')` → S2 widens the enum with **no
    migration churn**.
  - **Balance is DERIVED** — `SUM` over rows, not a stored running-balance column → no dual-write, and
    INV-5 durability falls out for free. Query written with the full `Σ(earn) − Σ(spend)` formula now,
    so S2 needs **zero balance-code change**. ❌ Rejected: a stored balance column.
  - **Atomicity:** `RecordTransaction` does account-lookup + `INSERT ... ON CONFLICT(ref) DO NOTHING`
    in **one `sql.Tx`**; `RowsAffected` `1 ⇒ created (201)`, `0 ⇒ replay (200)`. Single writer
    (`SetMaxOpenConns(1)`) serialises racing inserts → **INV-2**. This same tx is the seam S2 extends
    with the balance / no-negative check.
  - **Identity seam:** handlers read identity **only** via a `subjectAccountID(r)` resolver — S1 reads
    body/path, S3 swaps it to `r.Context()` + adds the 403 check. A **swap, not a rewrite** — makes
    INV-6 meaningful right now.
  - **Txn against an unknown account** → **404 `account_not_found`** (the FK backstops it).
- 🤖 **Did:** Read the brief PDF + `ARCHITECTURE`/`REST`/`ACCEPTANCE`/`SLICES` docs and the S0 code.
  Designed the REST cycle (`POST /accounts`, `GET /accounts/{id}`, `POST /transactions` (earn),
  `GET /accounts/{id}/balance`), the OpenAPI fragment (4 paths + `NewAccount`/`Account`/
  `NewTransaction`/`Transaction`/`Balance` schemas + stateful links), the goose migration
  `20260618120000_s1_accounts_and_transactions.sql` (accounts + transactions, `UNIQUE(ref)`,
  `points > 0` CHECK, `kind` CHECK), domain interfaces + sentinels, and the full red-test list.
  **Enriched GitHub issue #2** with the complete design (zero further design questions).
- ✅ **Accepted:** the full S1 design above, landed on issue #2. Invariants **INV-1/2/5/6** left
  unchanged in `ACCEPTANCE.md` — already registered for S1, status ⬜ (tests not written yet).
- 💡 **Why:** spec-first vertical slice; correctness comes from **SQL constraints + a single writer**,
  not app-level locking; concurrency tests ship *with* the slice, not bolted on later.
- 📚 `docs/specifications.pdf` (brief — final word) · `docs/ARCHITECTURE.md` ·
  `docs/REST_API_GUIDELINES.md` · `docs/ACCEPTANCE.md` · `docs/SLICES.md`.
- 🔗 **Artifacts:** GitHub issue #2 (enriched) — https://github.com/ossewawiel/gowallet/issues/2 ·
  planned migration `internal/sqlitestore/migrations/20260618120000_s1_accounts_and_transactions.sql` ·
  no code commits (design-only).

### ⏱️ 2026-06-18 · Entry 13 — S3 designed (Auth: JWT member/admin, design-only)

- 🧑 **Asked:** Run `/design-slice` for **S3** — Auth (JWT, member/admin). Design-only; enrich the
  existing GitHub issue **#4**, no production code.
- 🔎 **Explored / decisions weighed (and roads not taken):**
  - **Stateless JWT HS256 → no DB, no migration** for this slice. Auth is pure verification + a pure
    rule, so S3 carries **zero schema** and parallelizes straight off S0 (doesn't wait on S1's tables).
    Brief explicitly leaves the token scheme to us.
  - **`POST /token` is a demo token mint**, not a credential login — `{account_id, role}` → signed
    JWT. No password store is in scope (documented as a trade-off). `/token` is **DB-free** (no
    account-existence check), which keeps S3 independent of **S1**.
  - **Layer split stays at 3 packages:** verification (parse Bearer, pin HS256, extract claims) lives
    as `httpapi` **middleware**; the authorization rule (member-own vs admin-any) is a **pure function
    in `wallet`** — `Authorize(Identity, target) → ErrForbidden`. `Identity`/`Role` types live in
    `wallet`, so the domain owns the rule and the edge owns the crypto.
  - **Spec strategy:** a **global `security: [bearerAuth]`** default + per-op `security: []` opt-outs
    for `/token` and `/healthz`. Net effect: the moment S1/S2 endpoints enter the spec they're
    **protected by default** — no per-endpoint wiring to forget. New sentinel `ErrForbidden` → 403.
  - **Algorithm pinned** via `jwt.WithValidMethods(["HS256"])` → kills `alg:none` + RS↔HS confusion
    (INV-12).
  - **New config:** `GOWALLET_JWT_SECRET` (required, **fail-fast at boot**) + `GOWALLET_JWT_TTL`
    (default `1h`).
- 🤖 **Did:** Enriched **GitHub issue #4** with the full build spec — OpenAPI fragment, the "no
  migration" note, domain rules, the red-test list, and acceptance criteria. Added invariants
  **INV-12** (alg pinning) + **INV-13** (identity-from-token-only) to `docs/ACCEPTANCE.md`.
- ✅ **Accepted:** the full S3 design above. Invariants: **INV-7/INV-8** (pre-existing for S3) +
  **INV-12/INV-13** newly registered, all status ⬜.
- 💡 **Why:** stateless HS256 means a single service that signs *and* verifies — no key split, no DB,
  no migration; identity from the verified token only is what makes member-own enforcement real.
- 🔗 **Artifacts:** GitHub issue #4 (enriched) — https://github.com/ossewawiel/gowallet/issues/4 ·
  `docs/ACCEPTANCE.md` · branch `slice/s0-skeleton` · no code commits (design-only).

### ⏱️ 2026-06-18 · Entry 14 — S1 built (Accounts + Earn + Balance, red→green→quality-gate) ✅

- 🧑 **Asked:** Run `/build-slice 2` — build **S1** end-to-end with strict spec-first TDD via the
  `tdd-runner` subagent.
- 🔎 **Explored / decisions in-build:**
  - **Branch base correction:** local `main` was **2 commits stale** — S0 was in fact already merged
    to `origin/main` via **PR #7**. Branched `slice/s1-accounts-earn` off the up-to-date main (no
    re-merge needed) rather than off the stale local tip.
  - **Wired the `kin-openapi` request validator** (`internal/httpapi/validate.go`) — S0 hadn't needed
    it (only `/healthz`). **Scoped to spec routes**, infra routes untouched. Without it, Schemathesis
    caught `additionalProperties` **bypasses** slipping through.
  - **Spec tightened (spec-first, no behavior hacks)** to pass Schemathesis stateful:
    - documented **400** on the GET-by-id routes + a shared-envelope `ErrorHandlerFunc` for malformed
      path escapes;
    - `account_id` constrained to `^[A-Za-z0-9._-]+$`, `maxLength 64`, so ids round-trip cleanly as
      **path segments**;
    - `points` given `maximum` = int64-max so an over-`int64` value is **rejected at the edge**.
  - **Fixed a latent S0 spec bug:** `Error.message` description had an **unquoted comma** → YAML parsed
    a stray sibling key that `kin-openapi` rejected; now quoted (prose-only, **no codegen impact**).
  - **Atomicity landed as designed:** `sqlitestore.Store.RecordTransaction` does account-lookup +
    `INSERT ... ON CONFLICT(ref) DO NOTHING` + read-back in **one `sql.Tx`**; `RowsAffected`
    `1 ⇒ created (201)`, `0 ⇒ replay (200)`. Single writer serialises the race. **This is the seam S2
    extends.**
  - **Identity seam:** `subjectAccountID(r, candidate)` in `internal/httpapi/identity.go` — S1 returns
    the body/path candidate; **S3 swaps to `r.Context()`**. Handlers only ever call it.
  - **Balance is derived** (`SUM` over rows), not stored → **INV-5 durability is automatic**.
- 🤖 **Did:** spec-first **RED** (4 paths + 5 schemas + links in `api/openapi.yaml`, regen via
  `oapi-codegen`; failing unit/store/acceptance tests) → **GREEN** (migration
  `20260618120000_s1_accounts_and_transactions.sql`, `sqlc` queries + gen, `wallet` domain,
  `sqlitestore` impl, `httpapi` handlers, `main.go` wiring) → **REFACTOR** → **PROVE**. Installed
  **`sqlc` v1.31.1** (`go install`); used **MinGW gcc** for cgo/`-race` (system clang targets the MSVC
  ABI); Schemathesis needed `PYTHONUTF8=1` on Windows.
- ✅ **Accepted:** all of it — quality gate **green**, **INV-1/2/5/6 proven under `-race`**.
- 🧪 **Tests added:**
  - **unit** — `TestRecordEarn_NewRef_Created`, `TestRecordEarn_DuplicateRef_ReturnsExistingNotCounted`,
    `TestRecordEarn_UnknownAccount_NotFound`, `TestCreateAccount_DuplicateID_Conflict`,
    `TestGetAccount_Missing_NotFound`, `TestBalance_SumsEarns`;
  - **store** — `TestStore_InsertDuplicateRef_SecondIsNoOp`, `TestStore_Balance_DerivedFromRows`;
  - **acceptance** — `TestEarn_DuplicateRef_CountedOnce`, `TestEarn_ConcurrentSameRef_Once` (`-race`),
    `TestBalance_PersistsAcrossRestart`, `TestIsolation_NoCrossUserLeak` (`-race`).
- 💡 **Why:** correctness via **SQL constraints + single-writer**; concurrency proven **in-slice**;
  the spec stays the contract.
- 📚 GitHub issue #2 (design) · `docs/specifications.pdf` · `docs/ARCHITECTURE.md` ·
  `docs/REST_API_GUIDELINES.md` · `tdd-workflow` skill.
- 🔗 **Artifacts:** branch `slice/s1-accounts-earn` · commit `b6dd443` (`feat(s1): accounts + earn +
  balance`) · migration
  `internal/sqlitestore/migrations/20260618120000_s1_accounts_and_transactions.sql` · issue #2.

### ⏱️ 2026-06-18 · Entry 15 — S3 built (Auth: JWT member/admin, red→green→quality-gate) ✅

- 🧑 **Asked:** Run `/build-slice 4` — build **S3** (Auth) end-to-end with strict spec-first TDD via
  the `tdd-runner` subagent (red → green → refactor → prove).
- 🔎 **Explored / decisions in-build:**
  - **Stateless JWT HS256 → no DB, no migration.** Identity rides in the signed token; S3 carries
    **zero schema**. ❌ Rejected: a sessions/credentials table — out of scope, and the brief leaves
    the token scheme to us.
  - **Algorithm pinned** via `jwt.WithValidMethods(["HS256"])` → kills `alg:none` and RS↔HS
    confusion (**INV-12**). ❌ Rejected: accepting the token's self-declared `alg`.
  - **Layer split stays at 3 packages.** JWT *verification* (parse Bearer, pin HS256, extract
    claims) is `httpapi` **middleware**; the authorization *rule* is a **pure function in `wallet`**
    — `Authorize`, with `Identity`/`Role` types + an `ErrForbidden` sentinel. The domain owns the
    rule, the edge owns the crypto. ✅
  - **`role` kept as a `minLength:1` string, NOT a schema enum.** An *unknown* role must surface as a
    semantic **422** (handler's `ParseRole`); a schema enum would make `kin-openapi` bounce it as a
    **400** at the edge — wrong signal. ✏️ Side effect: the Schemathesis gate run uses
    `--exclude-checks positive_data_acceptance`, since a 422 on a *schema-valid-but-unknown* role is
    an intentional business rule, not a contract bug.
  - **Midstream S1 integration:** flipped the S1 handlers (`GetAccount`, `GetBalance`,
    `CreateTransaction`) from reading `account_id` in the body/URL to reading the **verified identity
    from request context** + `wallet.Authorize`. This is exactly the identity-seam swap S1 designed
    for (`subjectAccountID`) — a swap, not a rewrite — and it's what makes **INV-13** real.
  - **Latent issue flagged for a future slice:** a possible **int64 overflow** in the S1 balance
    `SUM` — noted out of S3 scope, to be handled where it belongs.
- 🤖 **Did:** spec-first **RED** (global `security: [bearerAuth]` default + `security: []` opt-outs,
  `POST /token`, regen via `oapi-codegen`; failing unit/acceptance tests) → **GREEN**
  (`internal/wallet/auth.go` — `Authorize`/`Identity`/`Role`/`ErrForbidden`; `internal/httpapi/auth.go`
  — verification middleware; `internal/httpapi/token.go` — demo token mint + `ParseRole`; S1 handler
  rewire) → **REFACTOR** → **PROVE**. **Config:** `GOWALLET_JWT_SECRET` (required, **fail-fast at
  boot**) + `GOWALLET_JWT_TTL` (default `1h`), read in `cmd/gowallet/main.go`. Added dep
  **`github.com/golang-jwt/jwt/v5`**.
- ✅ **Accepted:** all of it — quality gate **green** (gofmt · vet · golangci-lint clean ·
  `go build ./...` ok · `go test -race ./...` green · Schemathesis **1174 passed / 0 failures** ·
  boot fail-fast on missing secret verified). **INV-7/INV-8/INV-12/INV-13 proven.**
- 🧪 **Tests added:** `TestAccess_MemberOwnOnly`, `TestAccess_AdminAny`, `TestVerify_AlgNone_Rejected`,
  `TestVerify_NonHS256_Rejected`, `TestAuth_AlgConfusion_Rejected`, `TestAuth_IdentityFromTokenOnly`
  (+ supporting unit tests), landed in `test/acceptance/s3_auth_test.go` and the package test files.
- 💡 **Why:** a single service that signs *and* verifies needs no key split, no DB, no migration —
  HS256 + method pinning is simpler and dodges alg-confusion; identity from the **verified token
  only** is what makes member-own enforcement actually enforceable.
- 📚 GitHub issue #4 (design) · `docs/ACCEPTANCE.md` · `docs/ARCHITECTURE.md` (3-package layout,
  wire-crossing rule) · `tdd-workflow` skill.
- 🔗 **Artifacts:** branch `slice/s3-auth` · issue #4 · new files
  `internal/wallet/auth.go`, `internal/httpapi/auth.go`, `internal/httpapi/token.go` (+ tests) ·
  `test/acceptance/s3_auth_test.go` · `cmd/gowallet/main.go` (JWT config) · `go.mod`/`go.sum`
  (`golang-jwt/jwt/v5`). Closes #4 once the PR merges.

<!-- New entries go below this line -->
