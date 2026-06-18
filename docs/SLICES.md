# 🍰 Slice backlog

The build plan: **8 vertical slices**, each one full REST cycle. Ordered for **3 parallel streams**,
with **auth running parallel from right after S0** and **concurrency baked into the transaction
slices** (no separate hardening slice).

> 📂 Ready-to-paste kickoff prompts (one per slice) + the run-order guide live in
> [`docs/slices/`](slices/). Each slice also has a GitHub issue.

---

## 🗺️ The slices

| ID | Slice | REST cycle it delivers | Invariants | Depends on | Stream |
|----|-------|------------------------|------------|-----------|:------:|
| **S0** | Walking skeleton | `GET /healthz` + DB + migrations + `/openapi.yaml` + `/swagger` | — | — | ⛓️ serial |
| **S1** | Accounts + Earn + Balance | `POST /accounts`, `GET /accounts/{id}`, `POST /transactions`(earn), `GET …/balance` | INV-1,5 + 2,6 baked | S0 | 🅰️ |
| **S2** | Spend + guard | `POST /transactions`(spend), no-negative | INV-3 + 4 baked | S1 | 🅰️ |
| **S3** | Auth (JWT) | `POST /token`, verify middleware, member/admin | INV-7,8 | S0 | 🅱️ |
| **S4** | Audit trail | audit table + writer + `GET /audit` (admin) | INV-11, 21 | S0 | 🅲 |
| **S5** | CSV batch ingestion | ingest + summary, idempotent reprocess | INV-9,10 | S2, S4 | 🅲 |
| **S6** | Login (credential token) | `POST /login` → JWT; seed member+admin creds | INV-14–17 | S1, S3 | 🅱️ |
| **S7** | Listings | `GET /accounts` (admin), `GET /accounts/{id}/transactions` (own/admin) | INV-18–20 | S1, S3 | 🅰️ |

---

## 🔀 Dependency graph & the 3 streams

```
 SERIAL                  ┌──────────────── FAN OUT (max 3 parallel) ───────────────┐
   S0  ────────────────▶ │  🅰️  S1 ──▶ S2                                            │
   walking skeleton      │  🅱️  S3 (auth, integrates with S1/S2 midstream)          │
   (everyone waits)      │  🅲  S4 ──────────────────▶ S5  (waits on S2 + S4)        │
                         └──────────────────────────────────────────────────────────┘
```

- 🅰️ **Core spine:** `S0 → S1 → S2`. Serial within the stream (each builds on the last's data model).
  **S7 (listings)** extends the core reads (`GET /accounts`, per-account ledger) once S1 + S3 are in.
- 🅱️ **Security:** `S3` starts as soon as **S0** is in — JWT is pure crypto + HTTP, no DB dep. Its
  ownership enforcement wires across S1/S2 **midstream**. Then **S6 (login)** adds credential-based
  token issuance once **S1 + S3** are in.
- 🅲 **Ingestion + audit:** `S4` (audit) starts right after **S0**; `S5` (batch) waits for **S2 + S4**.

---

## ⛓️ Critical path & parallelism

- **Serial prefix = just S0.** After it merges, all three streams can run at once.
- **Critical path:** `S0 → S1 → S2 → S5` (S5 also needs S4, which runs in parallel on stream 🅲).
- **Max useful parallelism = 3** once S0 is merged.
- **Merge discipline:** timestamped goose migrations + rebase-before-merge keeps the streams from
  colliding (see `docs/DEVELOPMENT_FLOW.md`).

---

## 🏁 Order of attack

1. **S0** solo — unblocks everything.
2. Fan out to 3 streams: 🅰️ `S1 → S2` · 🅱️ `S3` · 🅲 `S4 → S5`.
3. **Integration + final test:** auth enforced across endpoints, then the full `docs/ACCEPTANCE.md`
   table + Schemathesis green = done.

> Each slice has a kickoff prompt in [`docs/slices/`](slices/) and a GitHub issue — **start with S0**.
