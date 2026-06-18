---
description: Run gowallet's full quality gate (format, vet, lint, build, race tests, Schemathesis) and report pass/fail.
argument-hint: (none)
---

Run the **gowallet quality gate** — the definition of "done" for a slice. Anchor on the
`tdd-workflow` skill and `docs/DEVELOPMENT_FLOW.md`.

Run each check, capture the output, and **don't stop at the first failure** — run them all so I see
the full picture:

| # | Check | Command |
|---|-------|---------|
| 1 | Format | `gofmt -l .`  (any output = unformatted files) |
| 2 | Vet | `go vet ./...` |
| 3 | Lint | `golangci-lint run` |
| 4 | Build | `go build ./...` |
| 5 | Tests + race | `go test -race ./...` |
| 6 | Contract | `schemathesis run http://localhost:8080/openapi.yaml` (start the server first if needed) |
| 7 | Invariants | confirm the relevant `docs/ACCEPTANCE.md` rows are ✅ |

> If a tool isn't installed yet (early in the project), mark that row ⏭️ skipped with a one-line note
> rather than failing the whole gate.

## Report back (house voice: visual, low lingo)
A single results table — ✅ / 🔴 / ⏭️ per check — then a one-line verdict (🟢 all green / 🔴 N
failing) and, for any 🔴, the exact failing output + the smallest next step.
