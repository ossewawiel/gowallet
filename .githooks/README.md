# Git hooks 🪝

Version-controlled hooks. **Enable them once per clone:**

```bash
git config core.hooksPath .githooks
```

(That's already set on the original dev machine; a fresh clone needs the one-liner above.)

---

## `pre-push` — SOLUTION.md stays in step with the code

`SOLUTION.md` is a graded deliverable, so it must reflect what actually shipped. This hook **blocks a
push** when the pushed commits change the API spec (`api/openapi.yaml`) or production Go
(`internal/**`, `cmd/**` — excluding `*_test.go` and generated `gen/`) **without** also touching
`SOLUTION.md`.

- 🔓 **Bypass one push** (use sparingly — e.g. a pure refactor that genuinely doesn't change the
  design): `git push --no-verify`  or  `SKIP_SOLUTION_CHECK=1 git push`.
- 🛟 **Fails open:** if it can't work out the commit range, it lets the push through rather than
  blocking you.

This is belt-and-suspenders on top of the quality-gate wiring (`CLAUDE.md`, the `doc-updater`
subagent, and `/build-slice`). The **gate reminds**; the **hook enforces**.
