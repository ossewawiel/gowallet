-- Schema for sqlc type inference only. The runtime source of truth is the
-- timestamped goose migrations in internal/sqlitestore/migrations/. Keep this
-- mirrored with 20260618120000_s1_accounts_and_transactions.sql.
CREATE TABLE accounts (
    account_id TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL UNIQUE,
    account_id  TEXT    NOT NULL REFERENCES accounts(account_id),
    kind        TEXT    NOT NULL CHECK (kind IN ('earn','spend')),
    points      INTEGER NOT NULL CHECK (points > 0),
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_transactions_account ON transactions(account_id);

-- Mirror of 20260618130000_s4_audit_log.sql for sqlc type inference. See the
-- migration for the rationale behind the deliberate differences from the
-- transactions table (ref NOT unique, no FK on account_id, no CHECK on
-- kind/points — the audit log records attempts as-attempted, append-only).
CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL,
    account_id  TEXT    NOT NULL,
    kind        TEXT    NOT NULL,
    points      INTEGER NOT NULL,
    outcome     TEXT    NOT NULL CHECK (outcome IN ('accepted','rejected','duplicate')),
    reason      TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_audit_account ON audit_log(account_id);
