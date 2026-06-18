// Command gowallet is the entrypoint. It is the ONE place that wires the three
// internal packages together: config → sqlitestore (open + migrate) → wallet
// (services) → httpapi (router) → http.ListenAndServe.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/sqlitestore"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// config is read from the environment with sane defaults. The only shared,
// process-wide state is this config plus the *sql.DB pool inside the store.
type config struct {
	addr     string
	dbPath   string
	specPath string
}

func loadConfig() config {
	return config{
		addr:     envOr("GOWALLET_ADDR", ":8080"),
		dbPath:   envOr("GOWALLET_DB", "gowallet.db"),
		specPath: envOr("GOWALLET_SPEC", "api/openapi.yaml"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := run(loadConfig()); err != nil {
		log.Fatalf("gowallet: %v", err)
	}
}

func run(cfg config) error {
	// Persistence: open with PRAGMAs, then apply migrations on startup.
	store, err := sqlitestore.Open(cfg.dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	specYAML, err := os.ReadFile(cfg.specPath)
	if err != nil {
		return err
	}

	// Transport: wire the health service (store satisfies wallet.Pinger) into
	// the router. httpapi never sees sqlitestore — only the wallet service.
	router := httpapi.NewRouter(httpapi.Deps{
		Health:   wallet.NewHealthService(store),
		Wallet:   wallet.NewWalletService(store, store),
		SpecYAML: specYAML,
	})

	log.Printf("gowallet listening on %s (db=%s)", cfg.addr, cfg.dbPath)
	if err := http.ListenAndServe(cfg.addr, router); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
