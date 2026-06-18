---
name: rest-api-standards
description: gowallet's REST API and error-handling conventions — resource naming, HTTP status codes, the single JSON error envelope (RFC 9457 style), idempotency via ref, JWT auth and role enforcement, and OpenAPI spec-first discipline. Use when designing endpoints, editing api/openapi.yaml, writing handlers, or shaping requests/responses and errors.
---

# REST API & error-handling standards

Working on **gowallet**'s HTTP surface. Anchor on `docs/REST_API_GUIDELINES.md`. Enforce:

## Spec-first (non-negotiable)
- `api/openapi.yaml` is the contract and the **source of truth**. Change the spec FIRST, regenerate
  with `oapi-codegen` (strict-server), then make code match. Never let code and spec drift.
- `kin-openapi` middleware validates requests against the spec at the edge — handlers assume valid input.

## Naming & methods
- Plural lowercase nouns: `/accounts`, `/transactions`. Ownership via hierarchy:
  `/accounts/{account_id}/balance`. No verbs in paths.
- Methods are the verbs: `GET` read, `POST` create/action, `PATCH` update, `DELETE` remove.

## Status codes (be honest)
- `201` on create (+ `Location`), `200` on read/action, `202` async (batch).
- `400/422` validation, `401` no/bad token, `403` not allowed, `404` missing, `409` business conflict
  (e.g. spend below zero), `500` we broke (never leak internals).
- **Idempotent replay is not an error**: re-POST of a known `ref` → `200` with the existing txn.

## One error envelope, everywhere
```json
{ "error": { "code": "insufficient_balance", "message": "human readable", "request_id": "req_..." } }
```
- `code` = stable snake_case machine string. `message` = friendly, never a stack trace/SQL.
- Map domain errors → status in ONE place. Always echo + log a `request_id`.

## Auth & access
- Bearer JWT (HS256, method pinned with `WithValidMethods(["HS256"])`). Identity from the **token**,
  via context — never the URL.
- `member` = own account only (else 403). `admin` = any account + adjustments.

## Conventions
- JSON only, UTF-8. Timestamps RFC 3339 UTC. Points are integers.
- Serve `/openapi.yaml` and `/swagger` (Swagger UI) for discovery + the demo.
