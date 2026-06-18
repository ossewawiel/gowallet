package wallet

import (
	"context"
	"errors"
	"time"
)

// Sentinel domain errors. The api layer maps these to HTTP status in ONE place
// (internal/httpapi). The store and service return these; handlers translate.
var (
	// ErrNotFound — the account (or its target) does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAccountExists — POST /accounts with an account_id already taken.
	ErrAccountExists = errors.New("account already exists")
	// ErrInvalidInput — defensive; most shape errors die at the spec edge.
	ErrInvalidInput = errors.New("invalid input")
	// ErrDuplicateRef is internal only: the store absorbs it into
	// created=false + the stored txn, so a replay never surfaces as a client
	// error (the handler returns 200). Kept for clarity at the store seam.
	ErrDuplicateRef = errors.New("duplicate ref")
	// ErrInsufficientBalance — a spend would drive the balance below zero. The
	// store's atomic guard returns this; the api layer maps it to 409.
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// Kind is the direction of a transaction. Points are always positive integers;
// the sign comes from the kind, not the value.
type Kind string

const (
	// KindEarn adds points. The only kind accepted by the S1 API.
	KindEarn Kind = "earn"
	// KindSpend subtracts points. The DB allows it; S2 opens the API to it.
	KindSpend Kind = "spend"
)

// Account is a member's wallet account. Pure Go — no DB types.
type Account struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Transaction is one ledger entry. Points are integers; OccurredAt is the
// client's business time (RFC 3339 UTC).
type Transaction struct {
	Ref        string
	AccountID  string
	Kind       Kind
	Points     int64
	OccurredAt time.Time
}

// AccountRepository is the wallet core's view of account persistence.
// sqlitestore implements it; httpapi never sees the implementation.
type AccountRepository interface {
	CreateAccount(ctx context.Context, a Account) error         // ErrAccountExists on dup id
	GetAccount(ctx context.Context, id string) (Account, error) // ErrNotFound
	Balance(ctx context.Context, id string) (int64, error)      // ErrNotFound if account absent
}

// TransactionRepository records transactions idempotently.
type TransactionRepository interface {
	// RecordTransaction inserts-or-replays atomically in ONE sql.Tx: it looks
	// up the account (→ ErrNotFound), runs INSERT ... ON CONFLICT(ref) DO
	// NOTHING, and returns the stored txn with created=false when the ref
	// already existed (idempotent replay, first-write-wins).
	RecordTransaction(ctx context.Context, t Transaction) (stored Transaction, created bool, err error)
}

// WalletService holds the core rules. It carries no per-request state — only
// the repositories (which wrap the shared *sql.DB pool). Safe to share.
type WalletService struct {
	accounts AccountRepository
	txns     TransactionRepository
}

// NewWalletService wires the service to its repositories.
func NewWalletService(accounts AccountRepository, txns TransactionRepository) *WalletService {
	return &WalletService{accounts: accounts, txns: txns}
}

// CreateAccount creates a member account. ErrAccountExists if the id is taken.
func (s *WalletService) CreateAccount(ctx context.Context, a Account) error {
	if a.ID == "" || a.Name == "" {
		return ErrInvalidInput
	}
	return s.accounts.CreateAccount(ctx, a)
}

// GetAccount reads one account. ErrNotFound if absent.
func (s *WalletService) GetAccount(ctx context.Context, id string) (Account, error) {
	return s.accounts.GetAccount(ctx, id)
}

// Balance returns the derived balance (Σ earn − Σ spend). ErrNotFound if the
// account does not exist.
func (s *WalletService) Balance(ctx context.Context, id string) (int64, error) {
	return s.accounts.Balance(ctx, id)
}

// RecordEarn records an earn transaction. It forces Kind=earn, delegates to the
// repository's atomic insert-or-replay, and returns created so the handler
// picks 201 (new) vs 200 (idempotent replay).
func (s *WalletService) RecordEarn(ctx context.Context, in Transaction) (Transaction, bool, error) {
	if in.Ref == "" || in.AccountID == "" || in.Points <= 0 {
		return Transaction{}, false, ErrInvalidInput
	}
	in.Kind = KindEarn
	return s.txns.RecordTransaction(ctx, in)
}

// RecordSpend records a spend transaction. It forces Kind=spend and delegates to
// the repository's atomic insert-then-check-and-rollback path, which returns
// ErrInsufficientBalance if the spend would drive the balance below zero. The
// service is a thin pass-through: the no-negative guard lives in ONE tx in the
// store (no read-then-write gap), so concurrent spends can't both pass a stale read.
func (s *WalletService) RecordSpend(ctx context.Context, in Transaction) (Transaction, bool, error) {
	if in.Ref == "" || in.AccountID == "" || in.Points <= 0 {
		return Transaction{}, false, ErrInvalidInput
	}
	in.Kind = KindSpend
	return s.txns.RecordTransaction(ctx, in)
}
