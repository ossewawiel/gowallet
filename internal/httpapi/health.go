package httpapi

import (
	"net/http"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// server implements gen.ServerInterface. It holds only the shared health
// service; identity and per-request data live in r.Context().
type server struct {
	health *wallet.HealthService
	wallet *wallet.WalletService
}

// GetHealth pings the database and reports readiness.
//   - DB up   → 200 {"status":"ok","db":"up"}
//   - DB down → 503 {"status":"degraded","db":"down"}
//
// The body is written as exact bytes (not struct-encoded) so the key order
// matches the contract Schemathesis fuzzes against.
func (s *server) GetHealth(w http.ResponseWriter, r *http.Request) {
	h := s.health.Check(r.Context())

	status := http.StatusOK
	if h.Status != "ok" {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"status":"` + h.Status + `","db":"` + h.DB + `"}`))
}
