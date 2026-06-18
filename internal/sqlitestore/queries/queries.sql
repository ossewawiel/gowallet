-- name: CreateAccount :exec
INSERT INTO accounts (account_id, name) VALUES (?, ?);

-- name: GetAccount :one
SELECT account_id, name, created_at FROM accounts WHERE account_id = ?;

-- name: AccountExists :one
SELECT EXISTS(SELECT 1 FROM accounts WHERE account_id = ?) AS present;

-- name: InsertTransaction :execresult
INSERT INTO transactions (ref, account_id, kind, points, occurred_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(ref) DO NOTHING;

-- name: GetTransactionByRef :one
SELECT ref, account_id, kind, points, occurred_at
FROM transactions
WHERE ref = ?;

-- name: BalanceForAccount :one
SELECT CAST(COALESCE(SUM(CASE WHEN kind = 'earn' THEN points ELSE -points END), 0) AS INTEGER) AS balance
FROM transactions
WHERE account_id = ?;

-- name: AppendAuditEntry :one
INSERT INTO audit_log (ref, account_id, kind, points, outcome, reason)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, ref, account_id, kind, points, outcome, reason, created_at;

-- name: ListAuditLog :many
SELECT id, ref, account_id, kind, points, outcome, reason, created_at
FROM audit_log
ORDER BY id DESC;

-- name: ListAuditLogByAccount :many
SELECT id, ref, account_id, kind, points, outcome, reason, created_at
FROM audit_log
WHERE account_id = ?
ORDER BY id DESC;
