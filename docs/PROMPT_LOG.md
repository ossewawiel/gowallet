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

<!-- New entries go below this line -->
