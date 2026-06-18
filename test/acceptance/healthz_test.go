package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/sqlitestore"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// acceptanceSecret is the HS256 signing secret used by every booted test app.
// Real config comes from GOWALLET_JWT_SECRET in cmd/gowallet; tests pin a known
// value so they can mint tokens via POST /token and exercise the auth path.
const acceptanceSecret = "acceptance-test-secret"

// bootRealApp wires the same three packages main.go does, against a real
// on-disk SQLite file, and returns a live test server.
func bootRealApp(t *testing.T) *httptest.Server {
	t.Helper()
	srv, _ := bootRealAppWithStore(t)
	return srv
}

// bootRealAppWithStore is bootRealApp plus the live *sqlitestore.Store, so
// acceptance tests can seed rows the API has no write path for (S4 audit: the
// log is written internally by AuditService, never by an HTTP client — tests
// seed via store.AppendAudit then assert via GET /audit).
func bootRealAppWithStore(t *testing.T) (*httptest.Server, *sqlitestore.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "acceptance.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	specYAML, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Health:    wallet.NewHealthService(store),
		Wallet:    wallet.NewWalletService(store, store),
		Audit:     wallet.NewAuditService(store),
		SpecYAML:  specYAML,
		JWTSecret: acceptanceSecret,
		JWTTTL:    time.Hour,
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, store
}

// authPostJSON POSTs a JSON body to url with a Bearer token.
func authPostJSON(t *testing.T, url, token string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// authBalance reads an account balance with the given bearer token, asserting 200.
func authBalance(t *testing.T, base, id, token string) int64 {
	t.Helper()
	resp := authGet(t, base, "/accounts/"+id+"/balance", token)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET balance %s: want 200, got %d", id, resp.StatusCode)
	}
	var got struct {
		AccountID string `json:"account_id"`
		Balance   int64  `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	return got.Balance
}

func TestHealthz_EndToEnd_PingsRealDB(t *testing.T) {
	srv := bootRealApp(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var got struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != "ok" || got.DB != "up" {
		t.Fatalf("real DB health: want {ok up}, got {%s %s}", got.Status, got.DB)
	}
}
