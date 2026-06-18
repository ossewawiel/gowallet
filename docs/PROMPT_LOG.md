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

<!-- New entries go below this line -->
