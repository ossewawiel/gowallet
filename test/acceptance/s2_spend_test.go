package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// spendBody builds a POST /transactions payload with kind=spend.
func spendBody(ref, id string, pts int64) map[string]any {
	return map[string]any{
		"ref":         ref,
		"account_id":  id,
		"kind":        "spend",
		"points":      pts,
		"occurred_at": "2024-06-01T10:00:00Z",
	}
}

// seedEarn earns points into an account over HTTP (setup for spend tests).
func seedEarn(t *testing.T, base, admin, ref, id string, pts int64) {
	t.Helper()
	resp := authPostJSON(t, base+"/transactions", admin, earnBody(ref, id, pts))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed earn %s: want 201, got %d", ref, resp.StatusCode)
	}
}

// A spend with enough balance → 201, body is the stored spend txn, balance drops.
func TestSpend_Succeeds_DecrementsBalance(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 200)

	resp := authPostJSON(t, srv.URL+"/transactions", admin, spendBody("spend-1", "member-1", 50))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("spend: want 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	if body["kind"] != "spend" {
		t.Fatalf("spend body kind: want spend, got %v", body["kind"])
	}

	if bal := getBalance(t, srv.URL, "member-1"); bal != 150 {
		t.Fatalf("balance after spend: want 150, got %d", bal)
	}
}

// INV-3: a spend that would go negative → 409 insufficient_balance, nothing written.
func TestSpend_BelowZero_Rejected(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 100)

	resp := authPostJSON(t, srv.URL+"/transactions", admin, spendBody("spend-1", "member-1", 150))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("over-spend: want 409, got %d", resp.StatusCode)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	_ = resp.Body.Close()
	if env.Error.Code != "insufficient_balance" {
		t.Fatalf("error code: want insufficient_balance, got %q", env.Error.Code)
	}

	if bal := getBalance(t, srv.URL, "member-1"); bal != 100 {
		t.Fatalf("balance after rejected spend: want 100 (unchanged), got %d", bal)
	}
}

// A re-POSTed spend ref → 201 then 200 with identical body; balance debited once.
func TestSpend_DuplicateRef_CountedOnce(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 100)

	first := authPostJSON(t, srv.URL+"/transactions", admin, spendBody("spend-dup", "member-1", 40))
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first spend: want 201, got %d", first.StatusCode)
	}
	var firstBody map[string]any
	_ = json.NewDecoder(first.Body).Decode(&firstBody)
	_ = first.Body.Close()

	second := authPostJSON(t, srv.URL+"/transactions", admin, spendBody("spend-dup", "member-1", 40))
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replay spend: want 200, got %d", second.StatusCode)
	}
	var secondBody map[string]any
	_ = json.NewDecoder(second.Body).Decode(&secondBody)
	_ = second.Body.Close()

	if fmt.Sprint(firstBody) != fmt.Sprint(secondBody) {
		t.Fatalf("replay body differs:\n first=%v\nsecond=%v", firstBody, secondBody)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 60 {
		t.Fatalf("balance after duplicate spend: want 60 (debited once), got %d", bal)
	}
}

// INV-4 (-race): 16 concurrent spends of 10 on a balance of 100 — exactly 10
// succeed (201), the rest 409; final balance is exactly 0, never negative.
func TestSpend_ConcurrentNoOverdraw(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 100)

	const n = 16
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := authPostJSON(t, srv.URL+"/transactions", admin,
				spendBody(fmt.Sprintf("spend-%d", i), "member-1", 10))
			codes[i] = resp.StatusCode
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	created, rejected := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			rejected++
		default:
			t.Fatalf("unexpected status under race: %d", c)
		}
	}
	if created != 10 || rejected != 6 {
		t.Fatalf("concurrent spends: want 10 created / 6 rejected, got %d / %d", created, rejected)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 0 {
		t.Fatalf("final balance after concurrent spends: want exactly 0, got %d", bal)
	}
}
