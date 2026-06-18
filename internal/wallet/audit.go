package wallet

import (
	"context"
	"time"
)

// AuditOutcome is OUR controlled vocabulary for how an attempt resolved. Unlike
// AuditEntry.Kind (free text — a faithful record of what was attempted, possibly
// invalid), the outcome is constrained: the writer rejects anything else, and
// the DB mirrors it with a CHECK.
type AuditOutcome string

const (
	// OutcomeAccepted — the attempt succeeded (e.g. a fresh earn/spend wrote).
	OutcomeAccepted AuditOutcome = "accepted"
	// OutcomeRejected — the attempt was refused (bad input, no account, …).
	OutcomeRejected AuditOutcome = "rejected"
	// OutcomeDuplicate — the ref was already seen (idempotent replay).
	OutcomeDuplicate AuditOutcome = "duplicate"
)

// valid reports whether o is one of the three known outcomes.
func (o AuditOutcome) valid() bool {
	switch o {
	case OutcomeAccepted, OutcomeRejected, OutcomeDuplicate:
		return true
	default:
		return false
	}
}

// AuditEntry is one recorded attempt. ID + CreatedAt are assigned by the store on
// append. Kind is free text: the attempted kind, possibly invalid on a rejected
// row — the audit log records what was attempted, not what's valid.
type AuditEntry struct {
	ID        int64
	Ref       string
	AccountID string
	Kind      string
	Points    int64
	Outcome   AuditOutcome
	Reason    string
	CreatedAt time.Time
}

// AuditRepository is append-only persistence for the audit log. sqlitestore
// implements it; httpapi never sees the implementation.
type AuditRepository interface {
	// AppendAudit inserts one row and returns it with the store-assigned id +
	// created_at. Append-only: no dedup, no upsert — the same ref appends again.
	AppendAudit(ctx context.Context, e AuditEntry) (AuditEntry, error)
	// ListAudit returns the log newest-first. accountID=="" → all; otherwise
	// only that account's rows.
	ListAudit(ctx context.Context, accountID string) ([]AuditEntry, error)
}

// AuditService is the append-only writer the transaction + batch paths (S5) call,
// plus the read used by GET /audit. It carries no per-request state — only the
// repository (which wraps the shared *sql.DB pool). Safe to share.
//
// The writer runs in its OWN insert, NEVER inside an earn/spend transaction: an
// audit failure can never roll back committed money. That's the whole reason S4
// keeps audit off the money path.
type AuditService struct {
	repo AuditRepository
}

// NewAuditService wires the service to its repository.
func NewAuditService(repo AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// Record appends one attempt. Append-only — every call inserts a new row. An
// unknown outcome is rejected with ErrInvalidInput (defensive; callers use the
// constants), and nothing is written.
func (s *AuditService) Record(ctx context.Context, e AuditEntry) (AuditEntry, error) {
	if !e.Outcome.valid() {
		return AuditEntry{}, ErrInvalidInput
	}
	return s.repo.AppendAudit(ctx, e)
}

// List returns the audit log newest-first. accountID=="" → the full log;
// otherwise only that account's attempts.
func (s *AuditService) List(ctx context.Context, accountID string) ([]AuditEntry, error) {
	return s.repo.ListAudit(ctx, accountID)
}
