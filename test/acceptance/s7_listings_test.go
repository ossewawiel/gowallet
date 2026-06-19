package acceptance_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// accountSummaryWire mirrors the AccountSummary schema on the wire.
type accountSummaryWire struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Balance   int64  `json:"balance"`
}

// ledgerEntryWire mirrors the LedgerEntry schema on the wire (NO account_id).
type ledgerEntryWire struct {
	Ref        string    `json:"ref"`
	Kind       string    `json:"kind"`
	Points     int64     `json:"points"`
	OccurredAt time.Time `json:"occurred_at"`
}

// listTransactions GETs the per-account ledger with the token and decodes the array.
func listTransactions(t *testing.T, base, accountID, token string) []ledgerEntryWire {
	t.Helper()
	resp := authGet(t, base, "/accounts/"+accountID+"/transactions", token)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET ledger %s: want 200, got %d", accountID, resp.StatusCode)
	}
	var out []ledgerEntryWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode ledger: %v", err)
	}
	return out
}

// TestListAccounts_AdminOnly (INV-18) — admin GET /accounts → 200 with the
// seeded accounts, each shaped {account_id,name,role,balance}.
func TestListAccounts_AdminOnly(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")

	admin := adminToken(t, srv.URL)
	resp := authGet(t, srv.URL, "/accounts", admin)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET /accounts: want 200, got %d", resp.StatusCode)
	}
	var out []accountSummaryWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	if len(out) < 2 {
		t.Fatalf("want >= 2 accounts, got %d", len(out))
	}
	for _, a := range out {
		if a.AccountID == "" || a.Name == "" || a.Role == "" {
			t.Fatalf("malformed summary: %+v", a)
		}
	}
}

// TestListAccounts_Member_Forbidden (INV-18) — member GET /accounts → 403.
func TestListAccounts_Member_Forbidden(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")

	token := mintToken(t, srv.URL, "member-1", "member")
	resp := authGet(t, srv.URL, "/accounts", token)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("member GET /accounts: want 403, got %d", resp.StatusCode)
	}
}

// TestListTransactions_MemberOwnOnly (INV-19) — member lists own ledger → 200,
// another's → 403.
func TestListTransactions_MemberOwnOnly(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "tx-1", "member-1", 100)

	token := mintToken(t, srv.URL, "member-1", "member")

	own := authGet(t, srv.URL, "/accounts/member-1/transactions", token)
	ownStatus := own.StatusCode
	_ = own.Body.Close()
	if ownStatus != http.StatusOK {
		t.Fatalf("member own ledger: want 200, got %d", ownStatus)
	}

	other := authGet(t, srv.URL, "/accounts/member-2/transactions", token)
	otherStatus := other.StatusCode
	_ = other.Body.Close()
	if otherStatus != http.StatusForbidden {
		t.Fatalf("member other ledger: want 403, got %d", otherStatus)
	}
}

// TestListTransactions_AdminAny (INV-19) — admin lists any account's ledger → 200.
func TestListTransactions_AdminAny(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "tx-1", "member-1", 100)

	resp := authGet(t, srv.URL, "/accounts/member-1/transactions", admin)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin any ledger: want 200, got %d", resp.StatusCode)
	}
}

// TestListTransactions_NoCrossAccountLeak (INV-20) — seed txns on two accounts;
// listing one returns ONLY its rows, in stable newest-first order.
func TestListTransactions_NoCrossAccountLeak(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")
	admin := adminToken(t, srv.URL)

	// Interleave inserts so id order (newest-first) is observable and distinct
	// from account order.
	seedEarn(t, srv.URL, admin, "m1-a", "member-1", 10)
	seedEarn(t, srv.URL, admin, "m2-a", "member-2", 99)
	seedEarn(t, srv.URL, admin, "m1-b", "member-1", 20)
	seedEarn(t, srv.URL, admin, "m1-c", "member-1", 30)

	got := listTransactions(t, srv.URL, "member-1", admin)
	if len(got) != 3 {
		t.Fatalf("member-1 ledger: want 3 rows, got %d", len(got))
	}
	// Only member-1's rows (no account_id on the wire, so assert by ref set).
	for _, e := range got {
		switch e.Ref {
		case "m1-a", "m1-b", "m1-c":
		default:
			t.Fatalf("cross-account leak: unexpected ref %q in member-1 ledger", e.Ref)
		}
	}
	// Newest-first, stable: insertion order was m1-a, m1-b, m1-c → reverse.
	if got[0].Ref != "m1-c" || got[1].Ref != "m1-b" || got[2].Ref != "m1-a" {
		t.Fatalf("newest-first: got %s, %s, %s", got[0].Ref, got[1].Ref, got[2].Ref)
	}
}

// TestListTransactions_MissingAccount_404 — admin lists a ghost account → 404
// (existence honesty: admin passes authz, so the store's 404 surfaces).
func TestListTransactions_MissingAccount_404(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	resp := authGet(t, srv.URL, "/accounts/ghost/transactions", admin)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("admin ghost ledger: want 404, got %d", resp.StatusCode)
	}
}

// TestListAccounts_BalanceMatchesDerived — the balance in the account list equals
// GET /accounts/{id}/balance for the same account (the reuse trap: same formula).
func TestListAccounts_BalanceMatchesDerived(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "tx-1", "member-1", 150)
	seedEarn(t, srv.URL, admin, "tx-2", "member-1", 25)

	want := authBalance(t, srv.URL, "member-1", admin)

	resp := authGet(t, srv.URL, "/accounts", admin)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /accounts: want 200, got %d", resp.StatusCode)
	}
	var out []accountSummaryWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	var got int64 = -1
	for _, a := range out {
		if a.AccountID == "member-1" {
			got = a.Balance
		}
	}
	if got != want {
		t.Fatalf("list balance %d != derived balance %d", got, want)
	}
}
