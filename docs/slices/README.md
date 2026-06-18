# 🍰 Slice prompts & run order

One **kickoff prompt** per slice. Each is a medium-rich starting point you paste into a **fresh
session** to drive `/design-slice` — which does the full design and enriches the matching GitHub
issue. Then `/build-slice <issue#>` builds it with strict TDD.

> High-level catalogue + dependency graph → [`../SLICES.md`](../SLICES.md). This file is the
> **operational** view: what to run, in what order, and where the issues are.

---

## ▶️ How to run a slice (fresh session each time)

1. Open a **fresh** Claude Code session in this repo.
2. Open the slice file (e.g. `S0.md`), copy the **Prompt** block, run it — it kicks off `/design-slice`.
3. `/design-slice` designs the slice and **enriches the existing GitHub issue** with the full spec
   (OpenAPI fragment, migration, red-test list).
4. Run `/build-slice <issue#>` → the `tdd-runner` builds it (🔴 red → 🟢 green → 🧹 refactor → 🧪 prove).
5. Open the PR (it `Closes #<issue#>`) and merge to `main`.

> Fresh session per slice keeps each build's context small and focused — that's the point of the
> issue-driven flow.

---

## 🗂️ The slices + their issues

| Order | Slice | Prompt | Issue | Stream | Depends on |
|:-----:|-------|--------|:-----:|:------:|-----------|
| 1️⃣ | S0 · Walking skeleton | [S0.md](S0.md) | [#1](https://github.com/ossewawiel/gowallet/issues/1) | ⛓️ serial | — |
| 2️⃣ | S1 · Accounts + Earn + Balance | [S1.md](S1.md) | [#2](https://github.com/ossewawiel/gowallet/issues/2) | 🅰️ | S0 |
| 3️⃣ | S2 · Spend + guard | [S2.md](S2.md) | [#3](https://github.com/ossewawiel/gowallet/issues/3) | 🅰️ | S1 |
| ⏩ | S3 · Auth (JWT) | [S3.md](S3.md) | [#4](https://github.com/ossewawiel/gowallet/issues/4) | 🅱️ | S0 |
| ⏩ | S4 · Audit trail | [S4.md](S4.md) | [#5](https://github.com/ossewawiel/gowallet/issues/5) | 🅲 | S0 |
| 🏁 | S5 · CSV batch ingestion | [S5.md](S5.md) | [#6](https://github.com/ossewawiel/gowallet/issues/6) | 🅲 | S2 + S4 |
| ⏩ | S6 · Login (credential token) | [S6.md](S6.md) | [#10](https://github.com/ossewawiel/gowallet/issues/10) | 🅱️ | S1 + S3 |

---

## 🔀 Run order (where the parallelism is)

```
 STEP 1 (serial):   S0 (#1)  ← do this first, alone — everything depends on it

 STEP 2 (fan out — up to 3 fresh sessions at once):
   🅰️ spine      S1 (#2) ──▶ S2 (#3)
   🅱️ security   S3 (#4) ──▶ S6 (#10) ← S3 auth (after S0); then S6 login (needs S1 + S3)
   🅲 ingestion  S4 (#5) ───────────▶ S5 (#6)    ← S5 waits for S2 (#3) AND S4 (#5)

 STEP 3 (integration + final test):
   auth enforced across endpoints · full docs/ACCEPTANCE.md table green · Schemathesis clean
```

**👉 Start here → [S0 / issue #1](https://github.com/ossewawiel/gowallet/issues/1).**
