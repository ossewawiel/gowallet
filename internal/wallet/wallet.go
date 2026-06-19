package wallet

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor for password hashes. 12 is a sensible 2020s
// default — strong enough to slow brute force, fast enough for a login path.
const bcryptCost = 12

// dummyHash is a valid bcrypt hash compared against on the account-not-found
// path so Login spends roughly the same time whether or not the account exists
// — flattening the timing channel that would otherwise enumerate accounts.
// It is bcrypt("flatten-timing", cost 12); the value is never a real secret.
const dummyHash = "$2a$12$LvEsOpgtZcSXg09X9vIgQeFp57IJxh35CAAcKhJJQExocBPl/Fta."

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
	// ErrInvalidCredentials — login failed: wrong secret OR unknown/secret-less
	// account. ONE error for all three so the api layer can't leak which (no
	// user enumeration).
	ErrInvalidCredentials = errors.New("invalid credentials")
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

// AccountSummary is one row of GET /accounts — an account plus its derived
// balance. It carries the role so the admin overview can show who's an admin.
type AccountSummary struct {
	ID      string
	Name    string
	Role    Role
	Balance int64
}

// AccountRepository is the wallet core's view of account persistence.
// sqlitestore implements it; httpapi never sees the implementation.
type AccountRepository interface {
	// CreateAccount stores the account plus an optional bcrypt passwordHash
	// ("" ⇒ NULL hash ⇒ the account can't log in). ErrAccountExists on dup id.
	CreateAccount(ctx context.Context, a Account, passwordHash string) error
	GetAccount(ctx context.Context, id string) (Account, error) // ErrNotFound
	Balance(ctx context.Context, id string) (int64, error)      // ErrNotFound if account absent
	// GetCredential returns the stored bcrypt hash + role for a login check. A
	// NULL hash comes back as "". ErrNotFound if the account is absent.
	GetCredential(ctx context.Context, id string) (hash string, role Role, err error)
	// ListAccounts returns every account with its derived balance (Σ earn −
	// Σ spend) — the SAME formula as Balance, so the list can't drift from
	// GET /balance. Ordered by account_id for a stable listing.
	ListAccounts(ctx context.Context) ([]AccountSummary, error)
	// ListTransactions returns the account's ledger newest-first. ErrNotFound if
	// the account does not exist (checked before listing, so a member's
	// cross-account 403 never leaks existence and an admin's ghost surfaces 404).
	ListTransactions(ctx context.Context, accountID string) ([]Transaction, error)
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

// CreateAccount creates a member account. If secret is non-empty it's hashed
// (bcrypt cost 12) and stored so the account can log in; an empty secret stores
// a NULL hash (account exists but can't log in). role is never taken from input
// — new accounts are always 'member' (admin is seed-only). ErrAccountExists if
// the id is taken.
func (s *WalletService) CreateAccount(ctx context.Context, a Account, secret string) error {
	if a.ID == "" || a.Name == "" {
		return ErrInvalidInput
	}
	var passwordHash string
	if secret != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(secret), bcryptCost)
		if err != nil {
			// bcrypt refuses secrets over 72 bytes — that's a bad request from
			// the client, not a server fault, so surface it as ErrInvalidInput
			// (→ 400 at the edge) rather than a 500.
			if errors.Is(err, bcrypt.ErrPasswordTooLong) {
				return ErrInvalidInput
			}
			return err
		}
		passwordHash = string(h)
	}
	return s.accounts.CreateAccount(ctx, a, passwordHash)
}

// Login verifies a credential and returns the identity to mint a token for.
// It returns the SAME ErrInvalidCredentials for an unknown account, a NULL/empty
// stored hash, and a wrong secret — so the api layer can never leak which case
// hit (no user enumeration). It runs a bcrypt compare even on the not-found and
// secret-less paths to flatten the timing channel.
func (s *WalletService) Login(ctx context.Context, accountID, secret string) (Identity, error) {
	hash, role, err := s.accounts.GetCredential(ctx, accountID)
	if err != nil || hash == "" {
		// Unknown account or no stored secret: still spend a compare so the
		// response time matches the wrong-secret path, then fail the same way.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(secret))
		return Identity{}, ErrInvalidCredentials
	}
	if cmpErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); cmpErr != nil {
		return Identity{}, ErrInvalidCredentials
	}
	return Identity{AccountID: accountID, Role: role}, nil
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

// ListAccounts returns every account with its derived balance (admin overview).
// Thin pass-through: the admin-only access gate lives in the handler, exactly
// like Balance/GetAccount.
func (s *WalletService) ListAccounts(ctx context.Context) ([]AccountSummary, error) {
	return s.accounts.ListAccounts(ctx)
}

// ListTransactions returns an account's ledger newest-first. ErrNotFound if the
// account is absent. Thin pass-through: the member-own/admin-any gate lives in
// the handler (it must run BEFORE this, so a cross-account member gets 403, not
// a 404 that would leak whether the account exists).
func (s *WalletService) ListTransactions(ctx context.Context, accountID string) ([]Transaction, error) {
	return s.accounts.ListTransactions(ctx, accountID)
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
