-- +goose Up
CREATE TABLE accounts (
    account_id TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL UNIQUE,                       -- INV-1/INV-2: same ref physically can't store twice
    account_id  TEXT    NOT NULL REFERENCES accounts(account_id),
    kind        TEXT    NOT NULL CHECK (kind IN ('earn','spend')),  -- CHECK allows spend now; API gates it to earn in S1
    points      INTEGER NOT NULL CHECK (points > 0),           -- integer points, direction comes from kind
    occurred_at TEXT    NOT NULL,                              -- client business time (RFC 3339)
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_transactions_account ON transactions(account_id);

-- +goose Down
DROP TABLE transactions;
DROP TABLE accounts;
