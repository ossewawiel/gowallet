# ✅ Acceptance registry — the testing source of truth (layer 2)

This is the **second half** of our source of truth. Layer 1 is `api/openapi.yaml` (shapes/contract,
fuzzed by Schemathesis). **Layer 2 is this file** — the *business invariants* and *concurrency
rules* the spec can't express. Every invariant here maps to a test that proves it.

**It grows per slice.** When a slice lands, its invariants move from ⬜ planned → ✅ proven, and
final testing = every row here is green at once.

> 🧠 Why two layers? An OpenAPI spec says "balance is an integer." It can't say "a spend can't drive
> balance below zero" or "two users at once never see each other's data." Those live here.

---

## 📜 Invariants from the brief (the non-negotiables)

| ID | Invariant | Proven by (test) | Slice | Status |
|----|-----------|------------------|-------|:------:|
| INV-1 | The same `ref`, submitted again, is **counted once** (idempotent) | `TestEarn_DuplicateRef_CountedOnce` | S1 | ⬜ |
| INV-2 | The same `ref` submitted **concurrently** still counts once | `TestEarn_ConcurrentSameRef_Once` (`-race`) | S1 | ⬜ |
| INV-3 | A spend that would make balance **negative is rejected** (409) | `TestSpend_BelowZero_Rejected` | S2 | ⬜ |
| INV-4 | **Concurrent spends** on one account never over-draw; final balance exact | `TestSpend_ConcurrentNoOverdraw` (`-race`) | S2 | ⬜ |
| INV-5 | Balance = sum(earns) − sum(spends), **durable across restart** | `TestBalance_PersistsAcrossRestart` | S1 | ⬜ |
| INV-6 | **No wire-crossing:** N users hitting their own accounts only ever see their own data | `TestIsolation_NoCrossUserLeak` (`-race`) | S1 | ⬜ |
| INV-7 | `member` can only touch **their own** account (else 403) | `TestAccess_MemberOwnOnly` | S3 | ⬜ |
| INV-8 | `admin` can view **any** account + apply adjustments | `TestAccess_AdminAny` | S3 | ⬜ |
| INV-9 | Batch ingest is **safe on reprocess** (same file twice = no double count) | `TestBatch_Reprocess_Idempotent` | S5 | ⬜ |
| INV-10 | Batch produces a **summary** (processed / accepted / rejected / duplicates) | `TestBatch_Summary` | S5 | ⬜ |
| INV-11 | Every batch attempt is **audited** with reason + timestamp | `TestAudit_RecordsEachAttempt` | S4 | ⬜ |

**Legend:** ⬜ planned · 🟡 test written (red) · ✅ proven (green in CI)

---

## 🧪 How each layer runs

| Layer | Tool | Catches | Command |
|-------|------|---------|---------|
| 1 — contract/shape | **Schemathesis** (property + stateful) | spec violations, crashes, bad status codes, sequence bugs | `schemathesis run http://localhost:8080/openapi.yaml` |
| 2 — invariants | **Go `testing` + `testify`**, `-race` | business rules, concurrency, wire-crossing | `go test -race ./test/acceptance/...` |

Schemathesis **stateful testing** chains operations via OpenAPI links (`POST /accounts → GET
/accounts/{id}`), so it exercises realistic sequences for free. The concurrency invariants
(INV-2/4/6) are the part Schemathesis can't reach — that's why they're explicit `-race` tests.

---

## ➕ Adding to this registry (per slice)

When you design a slice (`/design-slice`):
1. Add a row per new invariant (next `INV-n`), name the test, set the slice, status ⬜.
2. Copy those rows into the GitHub issue's "Invariants" section.
3. During the build, the test gets written (🟡), then passes (✅).
4. Final testing run = this whole table green + Schemathesis clean.
