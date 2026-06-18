# 🔄 Development flow

How a feature goes from idea → shipped, the gowallet way. Issue-driven, vertical slices, strict
TDD, proven against the spec. A fresh agent session should be able to read the issue + the skills
+ these docs and build the slice with **no further design questions**.

---

## 🍰 What's a "slice"?

A **vertical slice** = the *thinnest top-to-bottom path that makes one full REST cycle work and
testable*. Like a UI action end-to-end, but for the API. One slice touches every layer it needs:

```
spec fragment → migration → sqlc query → domain rule → handler → tests (unit + acceptance + contract)
```

A slice is **one capability**, not one function. "Earn points" (POST a txn + see the balance move +
idempotency) is a slice. Slices are listed and ordered in `docs/SLICES.md`.

---

## 🚥 The loop (per slice)

```
┌─────────────┐   ┌──────────────┐   ┌───────────────┐   ┌──────────────┐   ┌──────────────┐
│ 1. DESIGN   │──▶│ 2. RED        │──▶│ 3. GREEN      │──▶│ 4. REFACTOR  │──▶│ 5. PROVE     │
│ /design-    │   │ failing spec  │   │ minimal code  │   │ clean it up, │   │ Schemathesis │
│ slice → 🐙   │   │ + failing Go  │   │ to pass       │   │ tests stay   │   │ + -race +    │
│ issue       │   │ tests         │   │               │   │ green        │   │ ACCEPTANCE   │
└─────────────┘   └──────────────┘   └───────────────┘   └──────────────┘   └──────┬───────┘
                                                                                    │
                                                                          ┌─────────▼────────┐
                                                                          │ 6. LOG + PR      │
                                                                          │ PROMPT_LOG entry │
                                                                          │ → push → merge   │
                                                                          └──────────────────┘
```

### 1. Design → issue 🐙
Run `/design-slice`. We design it *here* (in the design session) and open a GitHub issue using the
**slice template** with everything an executor needs (spec fragment, data deltas, invariants, the
red-test list, acceptance criteria, depends-on, stream). The issue **is** the spec for the build.

### 2. Red 🔴 (spec-first — this is our TDD style)
- Add/extend the **OpenAPI fragment** in `api/openapi.yaml`. Regenerate (`oapi-codegen`).
- Write the **failing tests below it**: domain unit tests for the rule, an acceptance test for each
  invariant the issue lists. Run them → they **must fail** for the right reason.
- 🧠 *Why spec-first:* the spec defines the target; the failing Schemathesis/handler is the "red."

### 3. Green 🟢
Write the **minimum** code to pass — migration, sqlc query, domain rule, handler. Nothing fancy.
Resist building for slices that aren't here yet.

### 4. Refactor 🧹
Now make it clean: names, small functions, kill duplication. Tests stay green the whole time.

### 5. Prove 🧪 (the quality gate)
Everything in the gate must be green — see below. This is where Schemathesis fuzzes the endpoint
against the spec and the `-race` parallel tests prove no wire-crossing.

### 6. Log + ship 📦
Append a `docs/PROMPT_LOG.md` entry (use the **doc-updater** subagent). Update `ACCEPTANCE.md`
status. Push the branch, open a PR that **closes the issue**.

---

## 🌿 Branching & parallel streams (3 at once)

- **Branch per slice:** `slice/<id>-<short-name>` (e.g. `slice/s2-earn`). For truly parallel work,
  use a **git worktree** per branch so 3 streams don't trip over each other in one working dir.
- **Merge to `main` via PR.** PR closes its issue. `main` always stays green.
- **Migrations are TIMESTAMPED** (`goose create <name> sql` → `20260618153000_<name>.sql`). Two
  streams adding migrations won't collide on a sequence number; goose applies by timestamp order.
- **Rebase before merge** so migration order on `main` stays sane.

---

## ✅ Quality gate (the definition of done)

A slice isn't done until ALL of these are green (`/quality-gate` runs them):

| # | Check | Command |
|---|-------|---------|
| 1 | Formatted | `gofmt -l .` (empty = good) |
| 2 | Vet clean | `go vet ./...` |
| 3 | Lint clean | `golangci-lint run` |
| 4 | Builds | `go build ./...` |
| 5 | Tests + race | `go test -race ./...` |
| 6 | Contract | `schemathesis run` against the running server |
| 7 | Invariants | the slice's rows in `docs/ACCEPTANCE.md` pass |
| 8 | Logged | `docs/PROMPT_LOG.md` has the entry |

---

## 🔁 Feedback cadence

Every command reports back **visually** (frames, ✅/⚠️/🔴 status, a table of what ran). Never a
silent success. If a gate fails, say which check, the exact output, and the smallest next step.

---

## 🤝 Who does what

| Job | Who | Why |
|-----|-----|-----|
| Design a slice + open issue | main session (`/design-slice`) | needs the human in the loop |
| Build a slice (the loop) | `tdd-runner` subagent (`/build-slice`) | many tool calls — isolate context |
| Update PROMPT_LOG / SOLUTION / ACCEPTANCE | `doc-updater` subagent | large prose — keep main context lean |
| Scaffold files, `gh issue create` | inline | small, no isolation needed |
