# 🍰 Slice backlog

The build plan: each slice is one full REST cycle, shippable on its own. Ordered for **3 parallel
streams** with **auth coming in midstream** (not gating everything from day one).

---

## 🗺️ The slices

| ID | Slice | REST cycle it delivers | Key invariants | Depends on |
|----|-------|------------------------|----------------|-----------|
| **S0** | Walking skeleton | `GET /healthz` + DB connect + migrations run + `/openapi.yaml` + `/swagger` | — | — |
| **S1** | Accounts | `POST /accounts`, `GET /accounts/{id}` | — | S0 |
| **S2** | Earn + balance | `POST /transactions` (earn), `GET /accounts/{id}/balance` | INV-1, INV-5 | S1 |
| **S3** | Spend + guard | `POST /transactions` (spend), no-negative rule | INV-3 | S2 |
| **S4** | Concurrency hardening | WAL/busy_timeout + parallel-submission proofs | INV-2, INV-4, INV-6 | S3 |
| **S5** | Auth (JWT) | issue token, `member`/`admin`, enforce ownership | INV-7, INV-8 | S1 |
| **S6** | CSV batch ingestion | ingest endpoint/CLI + summary | INV-9, INV-10 | S2, S3 |
| **S7** | Audit trail | record each attempt (reason + timestamp) | INV-11 | S2 (table early) |

---

## 🔀 Dependency graph & the 3 streams

```
                 S0  (walking skeleton — everyone waits for this one)
                 │
                 ▼
                 S1  (accounts)
        ┌────────┼─────────────────────┐
        │        │                     │
   ▼ STREAM A   ▼ STREAM B        ▼ STREAM C
   (core spine) (security)        (ingestion + audit)
        │        │                     │
       S2        S5 ─ JWT issue/       S7 ─ audit table + writer
        │        │    verify +         │    (can start right after S1)
       S3        │    middleware,      │
        │        │    integrates       S6 ─ CSV ingest (needs S2+S3
       S4        │    across S1–S3          earn/spend logic)
                 │    midstream
                 ▼
            (merge auth into the spine once S3 is in)
```

**Reading it:**
- 🅰️ **Stream A — the spine:** `S0 → S1 → S2 → S3 → S4`. The heart of the wallet. Mostly serial
  because each builds on the last's data model.
- 🅱️ **Stream B — security:** `S5` starts as soon as **S1** exists (it needs accounts to attach
  roles to). JWT issue/verify + middleware get built in parallel, then wired across the endpoints
  **midstream** — exactly as you wanted, not blocking everything up front.
- 🅲 **Stream C — ingestion + audit:** `S7` (audit table + writer) can start right after **S1**.
  `S6` (CSV) needs the earn/spend service from **S2/S3**, so it lands a bit later but its parsing/
  summary scaffolding can be built ahead of time.

---

## ⛓️ Critical path & parallelism

- **Critical path:** `S0 → S1 → S2 → S3 → S4` (Stream A). Everything else hangs off S1/S2/S3.
- **Max useful parallelism = 3** once S1 is merged: A continues the spine, B does auth, C does audit/CSV.
- **Merge discipline:** timestamped goose migrations + rebase-before-merge keeps the 3 streams from
  colliding (see `docs/DEVELOPMENT_FLOW.md`).

---

## 🏁 Suggested order of attack

1. **S0** solo (unblocks all). 1 stream.
2. **S1** solo (unblocks B and C). 1 stream.
3. Fan out to **3 streams:** A=`S2→S3→S4`, B=`S5`, C=`S7→S6`.
4. **Integration pass:** merge auth across endpoints, run the full `docs/ACCEPTANCE.md` table +
   Schemathesis. That's the final-testing gate.

> Each slice gets its own GitHub issue via `/design-slice` before anyone writes code.
