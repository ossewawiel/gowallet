package wallet_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// fakeAuditRepo is an in-memory AuditRepository for the domain unit tests — no
// DB. It records every append so we can assert append-only behaviour and that a
// valid outcome passes straight through.
type fakeAuditRepo struct {
	appended []wallet.AuditEntry
	nextID   int64
}

func (f *fakeAuditRepo) AppendAudit(_ context.Context, e wallet.AuditEntry) (wallet.AuditEntry, error) {
	f.nextID++
	e.ID = f.nextID
	f.appended = append(f.appended, e)
	return e, nil
}

func (f *fakeAuditRepo) ListAudit(_ context.Context, accountID string) ([]wallet.AuditEntry, error) {
	if accountID == "" {
		return f.appended, nil
	}
	var out []wallet.AuditEntry
	for _, e := range f.appended {
		if e.AccountID == accountID {
			out = append(out, e)
		}
	}
	return out, nil
}

// TestAuditService_Record_ValidatesOutcome — an unknown outcome is rejected with
// ErrInvalidInput (defensive); a valid outcome passes straight through to the repo.
func TestAuditService_Record_ValidatesOutcome(t *testing.T) {
	ctx := context.Background()

	repo := &fakeAuditRepo{}
	svc := wallet.NewAuditService(repo)

	// Unknown outcome → ErrInvalidInput, nothing appended.
	_, err := svc.Record(ctx, wallet.AuditEntry{
		Ref: "tx-1", AccountID: "member-1", Kind: "earn", Points: 10,
		Outcome: wallet.AuditOutcome("bogus"), Reason: "x",
	})
	if !errors.Is(err, wallet.ErrInvalidInput) {
		t.Fatalf("unknown outcome: want ErrInvalidInput, got %v", err)
	}
	if len(repo.appended) != 0 {
		t.Fatalf("unknown outcome must not append: got %d rows", len(repo.appended))
	}

	// Each valid outcome passes straight through.
	for _, oc := range []wallet.AuditOutcome{
		wallet.OutcomeAccepted, wallet.OutcomeRejected, wallet.OutcomeDuplicate,
	} {
		if _, err := svc.Record(ctx, wallet.AuditEntry{
			Ref: "tx", AccountID: "member-1", Kind: "earn", Points: 1,
			Outcome: oc, Reason: "ok",
		}); err != nil {
			t.Fatalf("valid outcome %q: unexpected error %v", oc, err)
		}
	}
	if len(repo.appended) != 3 {
		t.Fatalf("valid outcomes: want 3 appends, got %d", len(repo.appended))
	}
}

// TestAuditService_Record_AppendsEveryCall — N calls produce N appends (no dedup,
// no upsert), even with the same ref. Append-only.
func TestAuditService_Record_AppendsEveryCall(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAuditRepo{}
	svc := wallet.NewAuditService(repo)

	const n = 5
	for i := 0; i < n; i++ {
		if _, err := svc.Record(ctx, wallet.AuditEntry{
			Ref: "same-ref", AccountID: "member-1", Kind: "earn", Points: 1,
			Outcome: wallet.OutcomeAccepted, Reason: "ok",
		}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	if len(repo.appended) != n {
		t.Fatalf("append-only: want %d appends, got %d", n, len(repo.appended))
	}
}
