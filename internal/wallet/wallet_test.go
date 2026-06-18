package wallet_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// fakeRepo is an in-memory AccountRepository + TransactionRepository used to
// unit-test the domain rules with no database. It mimics the atomicity
// contract: RecordTransaction is insert-or-replay keyed on ref.
type fakeRepo struct {
	accounts map[string]wallet.Account
	byRef    map[string]wallet.Transaction
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts: map[string]wallet.Account{},
		byRef:    map[string]wallet.Transaction{},
	}
}

func (f *fakeRepo) CreateAccount(_ context.Context, a wallet.Account) error {
	if _, ok := f.accounts[a.ID]; ok {
		return wallet.ErrAccountExists
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	f.accounts[a.ID] = a
	return nil
}

func (f *fakeRepo) GetAccount(_ context.Context, id string) (wallet.Account, error) {
	a, ok := f.accounts[id]
	if !ok {
		return wallet.Account{}, wallet.ErrNotFound
	}
	return a, nil
}

func (f *fakeRepo) Balance(_ context.Context, id string) (int64, error) {
	if _, ok := f.accounts[id]; !ok {
		return 0, wallet.ErrNotFound
	}
	var bal int64
	for _, t := range f.byRef {
		if t.AccountID != id {
			continue
		}
		switch t.Kind {
		case wallet.KindEarn:
			bal += t.Points
		case wallet.KindSpend:
			bal -= t.Points
		}
	}
	return bal, nil
}

func (f *fakeRepo) RecordTransaction(_ context.Context, t wallet.Transaction) (wallet.Transaction, bool, error) {
	if _, ok := f.accounts[t.AccountID]; !ok {
		return wallet.Transaction{}, false, wallet.ErrNotFound
	}
	if existing, ok := f.byRef[t.Ref]; ok {
		return existing, false, nil
	}
	f.byRef[t.Ref] = t
	return t, true, nil
}

func newService(t *testing.T) (*wallet.WalletService, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	return wallet.NewWalletService(repo, repo), repo
}

func mustAccount(t *testing.T, svc *wallet.WalletService, id string) {
	t.Helper()
	if err := svc.CreateAccount(context.Background(), wallet.Account{ID: id, Name: "T"}); err != nil {
		t.Fatalf("CreateAccount(%s): %v", id, err)
	}
}

func earn(ref, id string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindEarn,
		Points:     pts,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestRecordEarn_NewRef_Created(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	got, created, err := svc.RecordEarn(context.Background(), earn("tx-1", "member-1", 150))
	if err != nil {
		t.Fatalf("RecordEarn: %v", err)
	}
	if !created {
		t.Fatalf("created: want true, got false")
	}
	if got.Kind != wallet.KindEarn {
		t.Fatalf("kind: want earn, got %q", got.Kind)
	}
	if got.Points != 150 {
		t.Fatalf("points: want 150, got %d", got.Points)
	}
}

func TestRecordEarn_DuplicateRef_ReturnsExistingNotCounted(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	first, _, err := svc.RecordEarn(context.Background(), earn("tx-1", "member-1", 150))
	if err != nil {
		t.Fatalf("first RecordEarn: %v", err)
	}

	second, created, err := svc.RecordEarn(context.Background(), earn("tx-1", "member-1", 999))
	if err != nil {
		t.Fatalf("second RecordEarn: %v", err)
	}
	if created {
		t.Fatalf("created: want false on replay, got true")
	}
	if second.Points != first.Points {
		t.Fatalf("replay returned %d points, want stored %d (first-write-wins)", second.Points, first.Points)
	}
}

func TestRecordEarn_UnknownAccount_NotFound(t *testing.T) {
	svc, _ := newService(t)

	_, _, err := svc.RecordEarn(context.Background(), earn("tx-1", "ghost", 150))
	if !errors.Is(err, wallet.ErrNotFound) {
		t.Fatalf("err: want ErrNotFound, got %v", err)
	}
}

func TestCreateAccount_DuplicateID_Conflict(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	err := svc.CreateAccount(context.Background(), wallet.Account{ID: "member-1", Name: "Again"})
	if !errors.Is(err, wallet.ErrAccountExists) {
		t.Fatalf("err: want ErrAccountExists, got %v", err)
	}
}

func TestGetAccount_Missing_NotFound(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.GetAccount(context.Background(), "ghost")
	if !errors.Is(err, wallet.ErrNotFound) {
		t.Fatalf("err: want ErrNotFound, got %v", err)
	}
}

func TestBalance_SumsEarns(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	for i, pts := range []int64{100, 50, 25} {
		ref := "tx-" + string(rune('a'+i))
		if _, _, err := svc.RecordEarn(context.Background(), earn(ref, "member-1", pts)); err != nil {
			t.Fatalf("earn %d: %v", i, err)
		}
	}

	bal, err := svc.Balance(context.Background(), "member-1")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal != 175 {
		t.Fatalf("balance: want 175, got %d", bal)
	}
}
