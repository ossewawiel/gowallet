package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/sqlitestore"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// bootAppAt wires the full stack against a caller-chosen db path so a test can
// stop the server, reopen on the same file, and prove durability.
func bootAppAt(t *testing.T, dbPath string) (*httptest.Server, func()) {
	t.Helper()

	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatalf("Migrate: %v", err)
	}

	specYAML, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		_ = store.Close()
		t.Fatalf("read spec: %v", err)
	}

	svc := wallet.NewWalletService(store, store)
	router := httpapi.NewRouter(httpapi.Deps{
		Health:    wallet.NewHealthService(store),
		Wallet:    svc,
		SpecYAML:  specYAML,
		JWTSecret: acceptanceSecret,
		JWTTTL:    time.Hour,
	})
	srv := httptest.NewServer(router)

	stop := func() {
		srv.Close()
		_ = store.Close()
	}
	return srv, stop
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// adminToken mints an admin JWT — admin may act on any account, so test setup
// helpers (create account, read balance, seed earns) use it freely.
func adminToken(t *testing.T, base string) string {
	t.Helper()
	return mintToken(t, base, "test-admin", "admin")
}

func createAccount(t *testing.T, base, id string) {
	t.Helper()
	resp := authPostJSON(t, base+"/accounts", adminToken(t, base),
		map[string]any{"account_id": id, "name": "T-" + id})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create account %s: want 201, got %d", id, resp.StatusCode)
	}
}

func earnBody(ref, id string, pts int64) map[string]any {
	return map[string]any{
		"ref":         ref,
		"account_id":  id,
		"kind":        "earn",
		"points":      pts,
		"occurred_at": "2024-06-01T10:00:00Z",
	}
}

func getBalance(t *testing.T, base, id string) int64 {
	t.Helper()
	return authBalance(t, base, id, adminToken(t, base))
}

// INV-1: same ref resubmitted is counted once; replay returns 200 + same body.
func TestEarn_DuplicateRef_CountedOnce(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)

	first := authPostJSON(t, srv.URL+"/transactions", admin, earnBody("tx-1", "member-1", 150))
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first earn: want 201, got %d", first.StatusCode)
	}
	var firstBody map[string]any
	_ = json.NewDecoder(first.Body).Decode(&firstBody)
	_ = first.Body.Close()

	second := authPostJSON(t, srv.URL+"/transactions", admin, earnBody("tx-1", "member-1", 150))
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replay earn: want 200, got %d", second.StatusCode)
	}
	var secondBody map[string]any
	_ = json.NewDecoder(second.Body).Decode(&secondBody)
	_ = second.Body.Close()

	if fmt.Sprint(firstBody) != fmt.Sprint(secondBody) {
		t.Fatalf("replay body differs:\n first=%v\nsecond=%v", firstBody, secondBody)
	}

	if bal := getBalance(t, srv.URL, "member-1"); bal != 150 {
		t.Fatalf("balance after duplicate: want 150, got %d", bal)
	}
}

// INV-2 (-race): same ref submitted concurrently still counts once.
func TestEarn_ConcurrentSameRef_Once(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	admin := adminToken(t, srv.URL)

	const n = 16
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := authPostJSON(t, srv.URL+"/transactions", admin, earnBody("tx-race", "member-1", 100))
			codes[i] = resp.StatusCode
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusOK:
		default:
			t.Fatalf("unexpected status under race: %d", c)
		}
	}
	if created != 1 {
		t.Fatalf("concurrent same ref: want exactly 1 created (201), got %d", created)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 100 {
		t.Fatalf("balance after concurrent same ref: want 100, got %d", bal)
	}
}

// INV-5: balance durable across a restart (close store, reopen same file).
func TestBalance_PersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")

	srv1, stop1 := bootAppAt(t, dbPath)
	createAccount(t, srv1.URL, "member-1")
	resp := authPostJSON(t, srv1.URL+"/transactions", adminToken(t, srv1.URL), earnBody("tx-1", "member-1", 250))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("earn: want 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	if bal := getBalance(t, srv1.URL, "member-1"); bal != 250 {
		t.Fatalf("pre-restart balance: want 250, got %d", bal)
	}
	stop1()

	srv2, stop2 := bootAppAt(t, dbPath)
	defer stop2()
	if bal := getBalance(t, srv2.URL, "member-1"); bal != 250 {
		t.Fatalf("post-restart balance: want 250, got %d", bal)
	}
}

// INV-6 (-race): N members each earn to & read their OWN account; no cross-leak.
func TestIsolation_NoCrossUserLeak(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)

	const n = 12
	for i := 0; i < n; i++ {
		createAccount(t, srv.URL, fmt.Sprintf("member-%d", i))
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("member-%d", i)
			pts := int64((i + 1) * 10)
			resp := authPostJSON(t, srv.URL+"/transactions", admin, earnBody(fmt.Sprintf("tx-%d", i), id, pts))
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("earn for %s: want 201, got %d", id, resp.StatusCode)
			}
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("member-%d", i)
		want := int64((i + 1) * 10)
		if bal := getBalance(t, srv.URL, id); bal != want {
			t.Fatalf("%s balance: want %d (only its own earns), got %d", id, want, bal)
		}
	}
}
