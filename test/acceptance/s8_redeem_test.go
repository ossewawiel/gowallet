package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// redeemBody builds a POST /accounts/{id}/redeem payload. Note: NO account_id in
// the body — it's the path param, and identity is the token.
func redeemBody(ref, reward string, pts int64) map[string]any {
	return map[string]any{
		"ref":         ref,
		"points":      pts,
		"reward":      reward,
		"occurred_at": "2024-06-01T10:00:00Z",
	}
}

// redeem POSTs a redemption to the account's redeem endpoint with the token.
func redeem(t *testing.T, base, token, id, ref, reward string, pts int64) *http.Response {
	t.Helper()
	return authPostJSON(t, base+"/accounts/"+id+"/redeem", token, redeemBody(ref, reward, pts))
}

// INV-24: a redeem deducts points and counts against balance, surfacing in the
// ledger as kind=redeem.
func TestRedeem_DeductsFromBalance(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 200)

	resp := redeem(t, srv.URL, admin, "member-1", "rdm-1", "R150 voucher", 150)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("redeem: want 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	if body["kind"] != "redeem" {
		t.Fatalf("redeem body kind: want redeem, got %v", body["kind"])
	}

	if bal := getBalance(t, srv.URL, "member-1"); bal != 50 {
		t.Fatalf("balance after redeem: want 50, got %d", bal)
	}

	// Ledger shows the redeem row with kind=redeem.
	ledger := listTransactions(t, srv.URL, "member-1", admin)
	var sawRedeem bool
	for _, e := range ledger {
		if e.Ref == "rdm-1" && e.Kind == "redeem" && e.Points == 150 {
			sawRedeem = true
		}
	}
	if !sawRedeem {
		t.Fatalf("ledger missing redeem row: %+v", ledger)
	}
}

// INV-25: a redeem that would drive balance negative → 409 insufficient_balance;
// balance unchanged; nothing persisted (insert rolled back).
func TestRedeem_BelowZero_Rejected(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 100)

	resp := redeem(t, srv.URL, admin, "member-1", "rdm-1", "R150 voucher", 150)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("over-redeem: want 409, got %d", resp.StatusCode)
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
		t.Fatalf("balance after rejected redeem: want 100 (unchanged), got %d", bal)
	}
	// Nothing persisted: the rejected ref must not appear in the ledger.
	for _, e := range listTransactions(t, srv.URL, "member-1", admin) {
		if e.Ref == "rdm-1" {
			t.Fatalf("rejected redeem left a row behind: %+v", e)
		}
	}
}

// INV-26 (-race): N concurrent redeems of 100 on a balance of 100 → exactly one
// 201, the rest 409; final balance 0, never negative. Plus a redeem-vs-spend
// race that together would exceed balance → exactly one wins.
func TestRedeem_ConcurrentNoOverdraw(t *testing.T) {
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
			resp := redeem(t, srv.URL, admin, "member-1", fmt.Sprintf("rdm-%d", i), "R100", 100)
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
	if created != 1 || rejected != n-1 {
		t.Fatalf("concurrent redeems: want 1 created / %d rejected, got %d / %d", n-1, created, rejected)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 0 {
		t.Fatalf("final balance after concurrent redeems: want exactly 0, got %d", bal)
	}

	// redeem-vs-spend race: fresh account, balance 100, one redeem(100) + one
	// spend(100) fired together — together they'd overdraw, so exactly one wins.
	createAccount(t, srv.URL, "member-2")
	seedEarn(t, srv.URL, admin, "earn-2", "member-2", 100)

	var rwg sync.WaitGroup
	raceCodes := make([]int, 2)
	rwg.Add(2)
	go func() {
		defer rwg.Done()
		resp := redeem(t, srv.URL, admin, "member-2", "rdm-vs", "R100", 100)
		raceCodes[0] = resp.StatusCode
		_ = resp.Body.Close()
	}()
	go func() {
		defer rwg.Done()
		resp := authPostJSON(t, srv.URL+"/transactions", admin, spendBody("spend-vs", "member-2", 100))
		raceCodes[1] = resp.StatusCode
		_ = resp.Body.Close()
	}()
	rwg.Wait()

	wins := 0
	for _, c := range raceCodes {
		switch c {
		case http.StatusCreated:
			wins++
		case http.StatusConflict:
		default:
			t.Fatalf("redeem-vs-spend: unexpected status %d", c)
		}
	}
	if wins != 1 {
		t.Fatalf("redeem-vs-spend: want exactly 1 winner, got %d", wins)
	}
	if bal := getBalance(t, srv.URL, "member-2"); bal != 0 {
		t.Fatalf("redeem-vs-spend final balance: want 0, got %d", bal)
	}
}

// INV-27: redeem is member-own / admin-any. A member redeeming ANOTHER account's
// id → 403; member redeeming OWN → 201; admin redeeming ANY → 201.
func TestRedeem_MemberOwnOnly(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 500)
	seedEarn(t, srv.URL, admin, "earn-2", "member-2", 500)

	member1 := mintToken(t, srv.URL, "member-1", "member")

	// member-1 redeeming member-2's account → 403.
	cross := redeem(t, srv.URL, member1, "member-2", "rdm-cross", "R10", 10)
	if cross.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-account redeem: want 403, got %d", cross.StatusCode)
	}
	_ = cross.Body.Close()

	// member-1 redeeming own account → 201.
	own := redeem(t, srv.URL, member1, "member-1", "rdm-own", "R20", 20)
	if own.StatusCode != http.StatusCreated {
		t.Fatalf("own redeem: want 201, got %d", own.StatusCode)
	}
	_ = own.Body.Close()

	// admin redeeming any account → 201.
	any := redeem(t, srv.URL, admin, "member-2", "rdm-admin", "R30", 30)
	if any.StatusCode != http.StatusCreated {
		t.Fatalf("admin redeem: want 201, got %d", any.StatusCode)
	}
	_ = any.Body.Close()
}

// INV-27: the reward is recorded and returned. The 201 body echoes reward; the
// stored redeem row carries it (earn rows carry no reward).
func TestRedeem_RecordsReward(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 200)

	resp := redeem(t, srv.URL, admin, "member-1", "rdm-1", "R50 voucher", 50)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("redeem: want 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	if body["reward"] != "R50 voucher" {
		t.Fatalf("redeem body reward: want %q, got %v", "R50 voucher", body["reward"])
	}
	if body["account_id"] != "member-1" {
		t.Fatalf("redeem body account_id: want member-1, got %v", body["account_id"])
	}

	// Replay round-trips the same stored reward (200).
	replay := redeem(t, srv.URL, admin, "member-1", "rdm-1", "R50 voucher", 50)
	if replay.StatusCode != http.StatusOK {
		t.Fatalf("redeem replay: want 200, got %d", replay.StatusCode)
	}
	var replayBody map[string]any
	_ = json.NewDecoder(replay.Body).Decode(&replayBody)
	_ = replay.Body.Close()
	if replayBody["reward"] != "R50 voucher" {
		t.Fatalf("replay reward: want %q, got %v", "R50 voucher", replayBody["reward"])
	}
}

// INV-28: a redeem is idempotent on ref — POST the same ref twice → 201 then 200
// with the same stored redemption; balance deducted once.
func TestRedeem_DuplicateRef_CountedOnce(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)
	seedEarn(t, srv.URL, admin, "earn-1", "member-1", 200)

	first := redeem(t, srv.URL, admin, "member-1", "rdm-dup", "R40", 40)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first redeem: want 201, got %d", first.StatusCode)
	}
	var firstBody map[string]any
	_ = json.NewDecoder(first.Body).Decode(&firstBody)
	_ = first.Body.Close()

	second := redeem(t, srv.URL, admin, "member-1", "rdm-dup", "R40", 40)
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replay redeem: want 200, got %d", second.StatusCode)
	}
	var secondBody map[string]any
	_ = json.NewDecoder(second.Body).Decode(&secondBody)
	_ = second.Body.Close()

	if fmt.Sprint(firstBody) != fmt.Sprint(secondBody) {
		t.Fatalf("replay body differs:\n first=%v\nsecond=%v", firstBody, secondBody)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 160 {
		t.Fatalf("balance after duplicate redeem: want 160 (debited once), got %d", bal)
	}
}
