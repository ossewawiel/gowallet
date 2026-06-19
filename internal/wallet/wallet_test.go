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
	hashes   map[string]string
	byRef    map[string]wallet.Transaction
	order    []string // ref insertion order, so ListTransactions can be newest-first
	// recordErr, when non-nil, is returned by RecordTransaction for a fresh ref.
	// Lets a unit test drive the store-level error path (e.g. insufficient
	// balance) without a real DB — the service must pass it through untouched.
	recordErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts: map[string]wallet.Account{},
		hashes:   map[string]string{},
		byRef:    map[string]wallet.Transaction{},
	}
}

func (f *fakeRepo) CreateAccount(_ context.Context, a wallet.Account, passwordHash string) error {
	if _, ok := f.accounts[a.ID]; ok {
		return wallet.ErrAccountExists
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	f.accounts[a.ID] = a
	f.hashes[a.ID] = passwordHash
	return nil
}

// GetCredential satisfies wallet.AccountRepository. The base fakeRepo stores
// member-role accounts only; the credential variant (credRepo) overrides this.
func (f *fakeRepo) GetCredential(_ context.Context, id string) (string, wallet.Role, error) {
	h, ok := f.hashes[id]
	if !ok {
		return "", "", wallet.ErrNotFound
	}
	return h, wallet.RoleMember, nil
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

// ListAccounts / ListTransactions satisfy the extended AccountRepository. byRef
// is a map (unordered), so the fake replays f.order (insertion order) in reverse
// to mirror the real store's newest-first-by-id contract.

func (f *fakeRepo) ListAccounts(_ context.Context) ([]wallet.AccountSummary, error) {
	out := make([]wallet.AccountSummary, 0, len(f.accounts))
	for id, a := range f.accounts {
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
		out = append(out, wallet.AccountSummary{ID: id, Name: a.Name, Role: wallet.RoleMember, Balance: bal})
	}
	return out, nil
}

func (f *fakeRepo) ListTransactions(_ context.Context, accountID string) ([]wallet.Transaction, error) {
	if _, ok := f.accounts[accountID]; !ok {
		return nil, wallet.ErrNotFound
	}
	// Newest-first: f.order records insertion order; walk it in reverse.
	out := make([]wallet.Transaction, 0)
	for i := len(f.order) - 1; i >= 0; i-- {
		t := f.byRef[f.order[i]]
		if t.AccountID == accountID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) RecordTransaction(_ context.Context, t wallet.Transaction) (wallet.Transaction, bool, error) {
	if _, ok := f.accounts[t.AccountID]; !ok {
		return wallet.Transaction{}, false, wallet.ErrNotFound
	}
	if existing, ok := f.byRef[t.Ref]; ok {
		return existing, false, nil
	}
	if f.recordErr != nil {
		return wallet.Transaction{}, false, f.recordErr
	}
	f.byRef[t.Ref] = t
	f.order = append(f.order, t.Ref)
	return t, true, nil
}

func newService(t *testing.T) (*wallet.WalletService, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	return wallet.NewWalletService(repo, repo), repo
}

func mustAccount(t *testing.T, svc *wallet.WalletService, id string) {
	t.Helper()
	if err := svc.CreateAccount(context.Background(), wallet.Account{ID: id, Name: "T"}, ""); err != nil {
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

func spend(ref, id string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindSpend,
		Points:     pts,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

// RecordSpend must force Kind=spend regardless of what the caller passed in —
// the service owns the direction, not the request body.
func TestRecordSpend_ForcesKindSpend(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	in := wallet.Transaction{Ref: "tx-s", AccountID: "member-1", Kind: wallet.KindEarn, Points: 10,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)}
	got, _, err := svc.RecordSpend(context.Background(), in)
	if err != nil {
		t.Fatalf("RecordSpend: %v", err)
	}
	if got.Kind != wallet.KindSpend {
		t.Fatalf("kind: want spend, got %q", got.Kind)
	}
}

func TestRecordSpend_RejectsNonPositivePoints(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	_, _, err := svc.RecordSpend(context.Background(), spend("tx-s", "member-1", 0))
	if !errors.Is(err, wallet.ErrInvalidInput) {
		t.Fatalf("err: want ErrInvalidInput, got %v", err)
	}
}

// The service is a thin pass-through for the store's insufficient-balance verdict:
// the guard lives in the store (one tx), the service must not swallow or remap it.
func TestRecordSpend_PropagatesInsufficientBalance(t *testing.T) {
	repo := newFakeRepo()
	repo.recordErr = wallet.ErrInsufficientBalance
	svc := wallet.NewWalletService(repo, repo)
	if err := svc.CreateAccount(context.Background(), wallet.Account{ID: "member-1", Name: "T"}, ""); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, _, err := svc.RecordSpend(context.Background(), spend("tx-s", "member-1", 150))
	if !errors.Is(err, wallet.ErrInsufficientBalance) {
		t.Fatalf("err: want ErrInsufficientBalance (passed through), got %v", err)
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

	err := svc.CreateAccount(context.Background(), wallet.Account{ID: "member-1", Name: "Again"}, "")
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

// TestListAccounts_ReturnsSummariesWithBalance — the service maps store rows to
// summaries, each carrying its derived balance (Σ earn − Σ spend).
func TestListAccounts_ReturnsSummariesWithBalance(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")
	mustAccount(t, svc, "member-2")
	if _, _, err := svc.RecordEarn(context.Background(), earn("tx-1", "member-1", 150)); err != nil {
		t.Fatalf("earn: %v", err)
	}

	got, err := svc.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 summaries, got %d", len(got))
	}
	bal := map[string]int64{}
	for _, s := range got {
		bal[s.ID] = s.Balance
	}
	if bal["member-1"] != 150 {
		t.Fatalf("member-1 balance: want 150, got %d", bal["member-1"])
	}
	if bal["member-2"] != 0 {
		t.Fatalf("member-2 balance: want 0, got %d", bal["member-2"])
	}
}

// TestListTransactions_MissingAccount_NotFound — an unknown account yields ErrNotFound.
func TestListTransactions_MissingAccount_NotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.ListTransactions(context.Background(), "ghost")
	if !errors.Is(err, wallet.ErrNotFound) {
		t.Fatalf("err: want ErrNotFound, got %v", err)
	}
}

// TestListTransactions_NewestFirst — the returned slice is newest-first.
func TestListTransactions_NewestFirst(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")
	for _, ref := range []string{"tx-a", "tx-b", "tx-c"} {
		if _, _, err := svc.RecordEarn(context.Background(), earn(ref, "member-1", 10)); err != nil {
			t.Fatalf("earn %s: %v", ref, err)
		}
	}

	got, err := svc.ListTransactions(context.Background(), "member-1")
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 txns, got %d", len(got))
	}
	if got[0].Ref != "tx-c" || got[2].Ref != "tx-a" {
		t.Fatalf("newest-first: got order %s, %s, %s", got[0].Ref, got[1].Ref, got[2].Ref)
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
