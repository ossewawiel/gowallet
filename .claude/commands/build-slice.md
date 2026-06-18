---
description: Build a gowallet slice from its GitHub issue with strict TDD, via the tdd-runner subagent.
argument-hint: <github issue number, e.g. 7>
---

**Build the slice specified in GitHub issue #$ARGUMENTS** using strict, spec-first TDD.

## Flow

1. **Sanity check** — `gh issue view $ARGUMENTS`. Confirm it's a `slice` issue with the full spec
   (OpenAPI fragment, invariants, red-test list, depends-on). If its dependencies aren't merged to
   `main` yet, say so and stop.
2. **Delegate to the builder** — launch the **`tdd-runner`** subagent with issue number
   `$ARGUMENTS`. It runs the loop in its own context: branch → 🔴 red (spec + failing tests) →
   🟢 green (min code) → 🧹 refactor → 🧪 prove (full quality gate + Schemathesis + `-race`).
3. **On return:**
   - If 🔴 the runner hit a blocker — surface the exact failing check + output, propose the smallest fix.
   - If ✅ green — dispatch the **`doc-updater`** subagent to append the `PROMPT_LOG.md` entry and
     flip the slice's `ACCEPTANCE.md` rows to ✅.
4. **Offer the PR** — show the `gh pr create` command (PR body should `Closes #$ARGUMENTS`). Don't
   push or merge without my go-ahead.

## Report back (visual)
A status frame: slice id, branch, each quality-gate check ✅/🔴, tests added, files touched, and the
next step (open PR, or the blocker to clear).
