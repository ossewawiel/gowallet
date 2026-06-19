# 🎬 gowallet — demo & runbook

> A **guided tour** of the whole API: boot it, grab a token, then walk every endpoint — earn, spend,
> the no-negative guard, idempotency, batch ingest, audit, and the admin/member access split.
> Copy-paste friendly. ~5 minutes end to end.

---

## 0. 📦 Prerequisites

| Need | Version | Notes |
|------|---------|-------|
| **Go** | 1.26.x | that's the whole runtime — SQLite is pure-Go (`modernc.org/sqlite`), **no C compiler needed to run** |
| **curl** | any | the walkthrough uses it; Windows users can swap in `Invoke-RestMethod` (see the 💡 boxes) |
| **gcc (MinGW)** | only for `go test -race` | the race detector needs cgo; running the server does **not** |

No database to install, no services to start. `git clone && go run` and you're live.

---

## 1. 🚀 Boot the server

The server **refuses to start without a JWT secret** (a missing secret would silently disable auth —
we fail loud instead). On first boot it runs the goose migrations and **seeds two demo accounts**.

```bash
export GOWALLET_JWT_SECRET="demo-secret-please-change"   # required — any non-empty value
go run ./cmd/gowallet                                     # → "gowallet listening on :8080 (db=gowallet.db)"
```

> 💡 **PowerShell (Windows):**
> ```powershell
> $env:GOWALLET_JWT_SECRET = "demo-secret-please-change"
> go run ./cmd/gowallet
> ```

**Config knobs** (all optional except the secret):

| Env var | Default | What it does |
|---------|---------|--------------|
| `GOWALLET_JWT_SECRET` | — *(required)* | HMAC signing secret for HS256 JWTs |
| `GOWALLET_ADDR` | `:8080` | listen address |
| `GOWALLET_DB` | `gowallet.db` | SQLite file path (created + migrated on boot) |
| `GOWALLET_JWT_TTL` | `1h` | token lifetime (e.g. `30m`, `24h`) |
| `GOWALLET_SPEC` | `api/openapi.yaml` | the spec served at `/openapi.yaml` + validated against |

Confirm it's up:

```bash
curl -s localhost:8080/healthz      # → {"status":"ok","db":"up"}
```

Browse the live contract in your browser: **http://localhost:8080/swagger** 🧭

---

## 2. 🔑 Seeded demo accounts

> ⚠️ **Demo-only**, seeded by migration `20260619..._s6_account_credentials.sql` for local testing &
> grading. Secrets are stored as **bcrypt hashes**, never plaintext, and never returned by any endpoint.

| Role | `account_id` | `secret` | Can do |
|------|--------------|----------|--------|
| 👤 **member** | `member-123` | `demo-member-pw` | act on **its own** account only |
| 🛡️ **admin** | `admin-001` | `demo-admin-pw` | act on **any** account + batch + audit |

### Get a token

```bash
# Member token
MEMBER_TOK=$(curl -s -X POST localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"member-123","secret":"demo-member-pw"}' | jq -r .token)

# Admin token
ADMIN_TOK=$(curl -s -X POST localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"admin-001","secret":"demo-admin-pw"}' | jq -r .token)
```

> 💡 **PowerShell:**
> ```powershell
> $b = @{ account_id="admin-001"; secret="demo-admin-pw" } | ConvertTo-Json
> $ADMIN_TOK = (Invoke-RestMethod localhost:8080/login -Method Post -Body $b -ContentType application/json).token
> ```

A wrong secret **or** an unknown account returns an **identical `401`** — no way to tell which (no
user enumeration). Try it:

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/login \
  -H 'Content-Type: application/json' -d '{"account_id":"ghost","secret":"nope"}'   # → 401
```

---

## 3. 🧑‍💼 Create an account (member, with a login secret)

```bash
curl -s -X POST localhost:8080/accounts \
  -H "Authorization: Bearer $ADMIN_TOK" -H 'Content-Type: application/json' \
  -d '{"account_id":"member-777","name":"Sam","secret":"sams-pw"}'
# → 201 { "account_id":"member-777", "name":"Sam", "created_at":"..." }  + Location header
```

> `role` is **never** accepted here — new accounts are always `member` (admin is seed-only). The
> `secret` is optional; omit it and the account exists but can't log in.

---

## 4. 💰 Earn & spend (idempotent, with the no-negative guard)

Points are **integers**; the sign comes from `kind` (`earn` adds, `spend` subtracts).

```bash
# Earn 150 into member-777 (admin can act on any account)
curl -s -X POST localhost:8080/transactions \
  -H "Authorization: Bearer $ADMIN_TOK" -H 'Content-Type: application/json' \
  -d '{"ref":"tx-001","account_id":"member-777","kind":"earn","points":150,"occurred_at":"2026-06-19T10:00:00Z"}'
# → 201 (created)

# Re-POST the SAME ref → idempotent replay, counted once
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/transactions \
  -H "Authorization: Bearer $ADMIN_TOK" -H 'Content-Type: application/json' \
  -d '{"ref":"tx-001","account_id":"member-777","kind":"earn","points":150,"occurred_at":"2026-06-19T10:00:00Z"}'
# → 200 (replay, NOT a second +150)

# Spend 50
curl -s -X POST localhost:8080/transactions \
  -H "Authorization: Bearer $ADMIN_TOK" -H 'Content-Type: application/json' \
  -d '{"ref":"tx-002","account_id":"member-777","kind":"spend","points":50,"occurred_at":"2026-06-19T10:05:00Z"}'
# → 201, balance now 100

# Spend 9999 → would go negative → REJECTED
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/transactions \
  -H "Authorization: Bearer $ADMIN_TOK" -H 'Content-Type: application/json' \
  -d '{"ref":"tx-003","account_id":"member-777","kind":"spend","points":9999,"occurred_at":"2026-06-19T10:06:00Z"}'
# → 409 insufficient_balance  (nothing persists)
```

Check the balance (derived live as Σ earn − Σ spend):

```bash
curl -s localhost:8080/accounts/member-777/balance -H "Authorization: Bearer $ADMIN_TOK"
# → { "account_id":"member-777", "balance":100 }
```

---

## 5. 🔒 Access control in action (member vs admin)

```bash
# member-123's token reaching for member-777's balance → 403 (not its account)
curl -s -o /dev/null -w "%{http_code}\n" \
  localhost:8080/accounts/member-777/balance -H "Authorization: Bearer $MEMBER_TOK"   # → 403

# member-123 reading its OWN balance → 200
curl -s -o /dev/null -w "%{http_code}\n" \
  localhost:8080/accounts/member-123/balance -H "Authorization: Bearer $MEMBER_TOK"   # → 200

# No token at all → 401
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/accounts/member-777/balance    # → 401
```

> Identity always comes from the **verified token**, never the URL. A member token + a body/URL
> naming someone else's account is rejected — it can't grant a cross-account effect.

---

## 6. 📜 Listings (S7)

```bash
# Admin-only: every account with its derived balance
curl -s localhost:8080/accounts -H "Authorization: Bearer $ADMIN_TOK"
# → [ {"account_id":"admin-001","name":"...","role":"admin","balance":0}, ... ]

# A member hitting the list → 403
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/accounts -H "Authorization: Bearer $MEMBER_TOK"  # → 403

# Per-account ledger, newest-first (member-own / admin-any)
curl -s localhost:8080/accounts/member-777/transactions -H "Authorization: Bearer $ADMIN_TOK"
# → [ {"ref":"tx-002","kind":"spend","points":50,...}, {"ref":"tx-001","kind":"earn","points":150,...} ]
```

---

## 7. 📥 CSV batch ingest (admin)

`POST /batch` takes a `multipart/form-data` file (header `ref,account_id,kind,points,occurred_at`).
**Rejected rows are data, not errors** — they're tallied in the summary; only a broken *upload*
(missing/headerless file) is a `400`.

A ready-made fixture lives at **[`testdata/batch-member-123.csv`](../testdata/batch-member-123.csv)** —
7 rows against `member-123` (5 earns + 2 spends, net **+645**, never overdrawing) plus one for
`admin-001` and one for `member-777`:

```bash
curl -s -X POST localhost:8080/batch \
  -H "Authorization: Bearer $ADMIN_TOK" -F "file=@testdata/batch-member-123.csv"
# fresh DB (member-777 not created):
# → { "processed":9, "accepted":8, "rejected":1, "duplicates":0 }
#     rejected: seed-009 (member-777 doesn't exist yet — see note)
```

> 💡 `member-777` is **not** a seeded account — on a fresh DB its row is tallied as `rejected`
> (unknown account). Create it first (step 3 above) and it's accepted instead. `admin-001` and
> `member-123` are seeded, so their rows always land.

Re-uploading the **same file** is safe — every `ref` is already seen, so it comes back all
`duplicates`, no double-count:

```bash
curl -s -X POST localhost:8080/batch \
  -H "Authorization: Bearer $ADMIN_TOK" -F "file=@testdata/batch-member-123.csv"
# → { "processed":9, "accepted":0, "rejected":1, "duplicates":8 }

curl -s localhost:8080/accounts/member-123/balance -H "Authorization: Bearer $ADMIN_TOK"
# → { "account_id":"member-123", "balance":645 }   (unchanged by the reprocess)
```

---

## 8. 🕵️ Audit trail (admin)

Every transaction *attempt* (incl. each batch row) is recorded append-only with its outcome + reason.

```bash
# Full log, newest-first
curl -s localhost:8080/audit -H "Authorization: Bearer $ADMIN_TOK"

# Filtered to one account
curl -s "localhost:8080/audit?account_id=member-777" -H "Authorization: Bearer $ADMIN_TOK"

# A member hitting it → 403
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/audit -H "Authorization: Bearer $MEMBER_TOK"  # → 403
```

---

## 9. 🧪 Run the tests (the two-layer source of truth)

| Layer | What it proves | Command |
|-------|----------------|---------|
| **Invariants** (Go `-race`) | business rules, concurrency, no wire-crossing | `go test -race ./...` |
| **Contract** (Schemathesis) | every operation matches the spec (status/shape/sequences) | see below |

**Go tests** — `-race` needs cgo + a real gcc on the PATH:

```bash
CGO_ENABLED=1 go test -race ./...
```

> 💡 **Windows:** `$env:Path = "C:\Users\User-PC\scoop\apps\mingw\current\bin;$env:Path"; $env:CGO_ENABLED="1"; go test -race ./...`

**Schemathesis** — boot the server, log in as admin, fuzz the served spec with the token:

```powershell
$env:PYTHONUTF8 = "1"; $env:GOWALLET_JWT_SECRET = "schemathesis-secret"
# (boot the server in another shell, wait for /healthz)
$b = @{ account_id="admin-001"; secret="demo-admin-pw" } | ConvertTo-Json
$tok = (Invoke-RestMethod localhost:8080/login -Method Post -Body $b -ContentType application/json).token
schemathesis run "http://localhost:8080/openapi.yaml" -u "http://localhost:8080" `
  -H "Authorization: Bearer $tok" `
  --exclude-checks negative_data_rejection --exclude-checks positive_data_acceptance
```

> The two excluded checks cover **documented** stricter-than-schema validations (`/login` random creds
> → `401`, `/batch` headerless upload → `400`). Every structural check stays live and must pass.
> See `CLAUDE.md` → *Schemathesis MUST run with a Bearer token* for the full rationale.

Or run the whole gate in one shot: **`/quality-gate`** (a Claude command — see the README).

---

## 10. 🧹 Reset

The entire state is one SQLite file. To start fresh:

```bash
rm -f gowallet.db          # then boot again — migrations re-run, demo accounts re-seed
```

---

> 🗺️ Endpoint reference & error shapes → [`REST_API_GUIDELINES.md`](REST_API_GUIDELINES.md) ·
> architecture → [`ARCHITECTURE.md`](ARCHITECTURE.md) · the live OpenAPI → `GET /openapi.yaml`.
