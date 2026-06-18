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
