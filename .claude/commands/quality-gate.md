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
| 5 | Tests + race | `go test -race ./...` — **needs cgo + MinGW** (see below) |
| 6 | Contract | `schemathesis run` against the running server **with a Bearer token** (see below) |
| 7 | Invariants | confirm the relevant `docs/ACCEPTANCE.md` rows are ✅ |

> If a tool isn't installed yet (early in the project), mark that row ⏭️ skipped with a one-line note
> rather than failing the whole gate.

### 🪟 Windows specifics (use the PowerShell tool — full reference in `CLAUDE.md`)

**Step 5 — `-race`** is `CGO_ENABLED=0` by default but `-race` needs cgo. Prefix the run:
```powershell
$env:Path = "C:\Users\User-PC\scoop\apps\mingw\current\bin;$env:Path"; $env:CGO_ENABLED = "1"
go test -race ./...
```

**Step 6 — Schemathesis must carry a Bearer token** (protected routes 401 without one). Boot the
server, mint an **admin** token, run against the served spec:
```powershell
$env:PYTHONUTF8 = "1"; $env:GOWALLET_JWT_SECRET = "schemathesis-secret"
# boot ./cmd/gowallet (background), wait for /healthz, then:
$body = @{ account_id = "test-admin"; role = "admin" } | ConvertTo-Json
$tok  = (Invoke-RestMethod "http://localhost:8080/token" -Method Post -Body $body -ContentType "application/json").token
schemathesis run "http://localhost:8080/openapi.yaml" -u "http://localhost:8080" `
  -H "Authorization: Bearer $tok" --exclude-checks negative_data_rejection
```
> ⚠️ The single `POST /token` 422 finding (role validated semantically → 422, not a schema enum) is a
> **known, accepted non-issue** — not a gate failure. Every other operation must pass clean.

## Report back (house voice: visual, low lingo)
A single results table — ✅ / 🔴 / ⏭️ per check — then a one-line verdict (🟢 all green / 🔴 N
failing) and, for any 🔴, the exact failing output + the smallest next step.
