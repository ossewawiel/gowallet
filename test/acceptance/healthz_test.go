package acceptance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/sqlitestore"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// bootRealApp wires the same three packages main.go does, against a real
// on-disk SQLite file, and returns a live test server.
func bootRealApp(t *testing.T) *httptest.Server {
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
		Health:   wallet.NewHealthService(store),
		Wallet:   wallet.NewWalletService(store, store),
		SpecYAML: specYAML,
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
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
