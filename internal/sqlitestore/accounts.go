package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ossewawiel/gowallet/internal/sqlitestore/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// rfc3339 is the wire/storage time format (UTC). The DB stores RFC 3339 text;
// we parse it back into time.Time for the domain types.
const rfc3339 = "2006-01-02T15:04:05Z07:00"

// queries returns a sqlc Queries bound to the shared pool.
func (s *Store) queries() *gen.Queries { return gen.New(s.db) }

// CreateAccount inserts a new account. A duplicate account_id (PRIMARY KEY
// violation) maps to wallet.ErrAccountExists.
func (s *Store) CreateAccount(ctx context.Context, a wallet.Account) error {
	err := s.queries().CreateAccount(ctx, gen.CreateAccountParams{
		AccountID: a.ID,
		Name:      a.Name,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return wallet.ErrAccountExists
		}
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

// GetAccount reads one account. ErrNotFound if absent.
func (s *Store) GetAccount(ctx context.Context, id string) (wallet.Account, error) {
	row, err := s.queries().GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wallet.Account{}, wallet.ErrNotFound
		}
		return wallet.Account{}, fmt.Errorf("get account: %w", err)
	}
	created, _ := time.Parse(rfc3339, row.CreatedAt)
	return wallet.Account{ID: row.AccountID, Name: row.Name, CreatedAt: created.UTC()}, nil
}

// Balance returns the derived balance (Σ earn − Σ spend). ErrNotFound if the
// account does not exist (a missing account is distinct from a zero balance).
func (s *Store) Balance(ctx context.Context, id string) (int64, error) {
	q := s.queries()
	exists, err := q.AccountExists(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("account exists: %w", err)
	}
	if !exists {
		return 0, wallet.ErrNotFound
	}
	bal, err := q.BalanceForAccount(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("balance: %w", err)
	}
	return bal, nil
}

// RecordTransaction inserts-or-replays atomically in ONE sql.Tx. It confirms
// the account exists (→ ErrNotFound), then INSERT ... ON CONFLICT(ref) DO
// NOTHING. RowsAffected==1 ⇒ created; ==0 ⇒ replay → SELECT the stored row.
// The single writer (SetMaxOpenConns(1)) serialises racing inserts so exactly
// one wins; the losers read back the same stored txn. (INV-1, INV-2.)
func (s *Store) RecordTransaction(ctx context.Context, t wallet.Transaction) (wallet.Transaction, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	q := s.queries().WithTx(tx)

	exists, err := q.AccountExists(ctx, t.AccountID)
	if err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("account exists: %w", err)
	}
	if !exists {
		return wallet.Transaction{}, false, wallet.ErrNotFound
	}

	res, err := q.InsertTransaction(ctx, gen.InsertTransactionParams{
		Ref:        t.Ref,
		AccountID:  t.AccountID,
		Kind:       string(t.Kind),
		Points:     t.Points,
		OccurredAt: t.OccurredAt.UTC().Format(rfc3339),
	})
	if err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("insert transaction: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("rows affected: %w", err)
	}

	// No-negative guard, baked into the SAME tx (INV-3/INV-4). On a FRESH spend
	// insert (affected==1, kind==spend) the just-inserted row is already counted
	// by BalanceForAccount; if that drives the balance below zero we roll back
	// (undoing the insert) and report ErrInsufficientBalance — nothing persists.
	// A replay (affected==0) skips the check: the first write already validated
	// it (first-write-wins), so re-checking would be wrong and a wasted read.
	// Earn can't go negative, so it skips the check too. The single writer
	// serialises racing spends, so each sees every committed spend before it —
	// two concurrent spends can't both pass a stale read.
	if affected == 1 && t.Kind == wallet.KindSpend {
		bal, balErr := q.BalanceForAccount(ctx, t.AccountID)
		if balErr != nil {
			return wallet.Transaction{}, false, fmt.Errorf("balance check: %w", balErr)
		}
		if bal < 0 {
			if rbErr := tx.Rollback(); rbErr != nil {
				return wallet.Transaction{}, false, fmt.Errorf("rollback spend: %w", rbErr)
			}
			return wallet.Transaction{}, false, wallet.ErrInsufficientBalance
		}
	}

	// Read the stored row inside the same tx — for a fresh insert it's our row;
	// for a replay it's the first writer's row (first-write-wins).
	row, err := q.GetTransactionByRef(ctx, t.Ref)
	if err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("get transaction by ref: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return wallet.Transaction{}, false, fmt.Errorf("commit: %w", err)
	}

	occurred, _ := time.Parse(rfc3339, row.OccurredAt)
	stored := wallet.Transaction{
		Ref:        row.Ref,
		AccountID:  row.AccountID,
		Kind:       wallet.Kind(row.Kind),
		Points:     row.Points,
		OccurredAt: occurred.UTC(),
	}
	return stored, affected == 1, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE/PRIMARY KEY
// constraint failure. modernc.org/sqlite surfaces it in the message text.
func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT FAILED")
}
