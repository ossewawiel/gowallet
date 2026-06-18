package sqlitestore

import (
	"context"
	"fmt"
	"time"

	"github.com/ossewawiel/gowallet/internal/sqlitestore/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// AppendAudit inserts one append-only audit row and returns it with the
// DB-assigned id + created_at (via RETURNING, one round-trip). It runs as its
// OWN statement — never inside an earn/spend tx — so an audit write can't roll
// back committed money (INV-11, the trap S4 guards against).
func (s *Store) AppendAudit(ctx context.Context, e wallet.AuditEntry) (wallet.AuditEntry, error) {
	row, err := s.queries().AppendAuditEntry(ctx, gen.AppendAuditEntryParams{
		Ref:       e.Ref,
		AccountID: e.AccountID,
		Kind:      e.Kind,
		Points:    e.Points,
		Outcome:   string(e.Outcome),
		Reason:    e.Reason,
	})
	if err != nil {
		return wallet.AuditEntry{}, fmt.Errorf("append audit: %w", err)
	}
	return auditFromRow(row), nil
}

// ListAudit returns the audit log newest-first (ORDER BY id DESC — id is
// strictly monotonic, unlike second-precision created_at). accountID=="" → the
// full log; otherwise only that account's rows (no cross-account leak).
func (s *Store) ListAudit(ctx context.Context, accountID string) ([]wallet.AuditEntry, error) {
	q := s.queries()

	var rows []gen.AuditLog
	var err error
	if accountID == "" {
		rows, err = q.ListAuditLog(ctx)
	} else {
		rows, err = q.ListAuditLogByAccount(ctx, accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}

	out := make([]wallet.AuditEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditFromRow(row))
	}
	return out, nil
}

// auditFromRow maps a sqlc audit_log row to the domain type, parsing the stored
// RFC 3339 text back into time.Time.
func auditFromRow(row gen.AuditLog) wallet.AuditEntry {
	created, _ := time.Parse(rfc3339, row.CreatedAt)
	return wallet.AuditEntry{
		ID:        row.ID,
		Ref:       row.Ref,
		AccountID: row.AccountID,
		Kind:      row.Kind,
		Points:    row.Points,
		Outcome:   wallet.AuditOutcome(row.Outcome),
		Reason:    row.Reason,
		CreatedAt: created.UTC(),
	}
}
