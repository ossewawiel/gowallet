# 🌐 REST API guidelines

The house style for endpoints, status codes, errors, and idempotency. The OpenAPI spec
(`api/openapi.yaml`) is the contract; this doc is the *taste* behind it. Keep them in agreement.

---

## 🧭 Resource naming

- **Nouns, plural, lowercase:** `/accounts`, `/transactions`. No verbs in paths (`/createAccount` ❌).
- **Hierarchy for ownership:** `/accounts/{account_id}/balance`.
- **Let HTTP methods be the verbs:**

| Method | Meaning | Example |
|--------|---------|---------|
| `GET` | read, no side effects | `GET /accounts/{id}` |
| `POST` | create / submit an action | `POST /transactions` |
| `PUT`/`PATCH` | replace / partial update | `PATCH /accounts/{id}` (admin) |
| `DELETE` | remove | (not used yet) |

---

## 🚦 Status codes (pick the honest one)

| Code | When |
|------|------|
| `200 OK` | successful read / action with a body |
| `201 Created` | a resource was created (return it + `Location`) |
| `202 Accepted` | async accepted (batch ingestion kicked off) |
| `400 Bad Request` | malformed body / failed validation |
| `401 Unauthorized` | missing/invalid token |
| `403 Forbidden` | valid token, **not allowed** (member touching another's account) |
| `404 Not Found` | resource doesn't exist |
| `409 Conflict` | business conflict (e.g. spend would go negative) |
| `422 Unprocessable` | well-formed but semantically invalid |
| `500` | we broke — never leak internals |

> 💡 **Idempotent replay isn't an error.** Re-POSTing the same `ref` returns **`200`** with the
> existing transaction (or `409` if you prefer strict) — *not* a double-spend. Decide per endpoint
> in the spec and keep it consistent.

---

## 💥 Error shape (one envelope, everywhere)

Every error returns the **same JSON shape** so clients can rely on it. We follow the spirit of
**RFC 9457 (problem details)**, trimmed:

```json
{
  "error": {
    "code": "insufficient_balance",
    "message": "spend of 200 would drive balance below zero (current: 150)",
    "request_id": "req_01H..."
  }
}
```

- `code` is a **stable machine string** (snake_case), safe to switch on.
- `message` is human-friendly; never a raw stack trace or SQL error.
- `request_id` ties the response to the logs.

**Domain errors → status mapping** (one place, in the api layer):

| Domain error | HTTP |
|--------------|------|
| `ErrNotFound` | 404 |
| `ErrDuplicateRef` | 200/409 (idempotent replay) |
| `ErrInsufficientBalance` | 409 |
| `ErrForbidden` | 403 |
| `ErrInvalidInput` | 400/422 |

---

## 🔁 Idempotency (the `ref` contract)

- Every transaction carries a client-supplied **`ref`**. It's the idempotency key.
- The store has `UNIQUE(ref)`. Submitting a known `ref` **never** applies twice.
- This must hold **under concurrency** — two identical `ref`s racing → exactly one wins, the other
  gets the idempotent response. Proven by a parallel-submission test.

---

## 🔐 Auth & access rules

- **Bearer JWT:** `Authorization: Bearer <token>`. Verified by middleware (HS256, method pinned).
- **Identity comes from the token, never the URL.** A handler reads `account_id`/`role` from
  `r.Context()`. (This is the wire-crossing guarantee in practice.)
- **Roles:**
  - `member` → may read **their own** balance and submit **their own** earn/spend.
  - `admin` → may read **any** account and apply adjustments.
- Member touching someone else's account → **403**, not 404 (don't leak existence... unless the
  spec says otherwise — keep it consistent).

---

## 📦 Request/response conventions

- **JSON only**, `Content-Type: application/json`. UTF-8.
- **Timestamps:** RFC 3339 / ISO 8601 UTC (`2024-06-01T10:00:00Z`) — matches the brief's examples.
- **Points are integers.** No floats for money/points, ever.
- **Validate at the edge:** `kin-openapi` middleware rejects anything off-spec before it reaches a
  handler. Handlers can assume well-formed input.
- **Always echo a `request_id`** (generate one in middleware if absent) and log it.

---

## 📖 Discoverability

- `GET /openapi.yaml` serves the live spec.
- `GET /swagger` serves **Swagger UI** — click-to-test every endpoint. This is the human demo
  surface (and what the Loom walks through).
