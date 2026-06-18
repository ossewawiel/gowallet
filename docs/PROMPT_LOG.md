# 🗂️ Prompt & Decision Timeline

A running, plain-language log of how gowallet was built — the prompts asked, what the AI did,
what was accepted or edited, and **why**. Newest entries at the bottom. This is the "director's
commentary" for [`SOLUTION.md`](../SOLUTION.md).

**Legend:** 🧑 me · 🤖 assistant · ✅ accepted · ✏️ edited/steered · 💡 rationale · 📚 source

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

<!-- New entries go below this line -->
