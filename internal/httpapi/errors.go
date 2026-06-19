package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// writeDomainError is the ONE place domain sentinels map to HTTP. Every handler
// funnels its errors through here so status codes + envelope codes stay
// consistent across the whole API (per docs/REST_API_GUIDELINES.md).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, wallet.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "you may not access this account")
	case errors.Is(err, wallet.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, wallet.ErrAccountExists):
		writeError(w, r, http.StatusConflict, "account_exists", "account_id already exists")
	case errors.Is(err, wallet.ErrInsufficientBalance):
		writeError(w, r, http.StatusConflict, "insufficient_balance", "transaction would drive balance below zero")
	case errors.Is(err, wallet.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "request body is invalid")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

// writeError emits the single JSON error envelope used across the whole API
// (per docs/REST_API_GUIDELINES.md). One place, so every slice stays
// consistent. code is a stable snake_case machine string; message is
// human-friendly and must never leak a stack trace or SQL.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	var env gen.Error
	env.Error.Code = code
	env.Error.Message = message
	if rid := middleware.GetReqID(r.Context()); rid != "" {
		env.Error.RequestId = &rid
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// notFoundHandler answers unknown paths with the shared error envelope.
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
}

// methodNotAllowedHandler answers a known path hit with the wrong method. It
// guarantees an Allow header (RFC 9110): chi records the matched methods on the
// header before calling us; if absent we fall back to GET (every route is GET).
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	if w.Header().Get("Allow") == "" {
		w.Header().Set("Allow", http.MethodGet)
	}
	writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

// recoverer turns a panic in any handler into a clean 500 + error envelope
// instead of a dropped connection. Wraps chi's request-id so the id is echoed.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("httpapi: recovered panic on %s %s: %v", r.Method, r.URL.Path, rec)
				writeError(w, r, http.StatusInternalServerError,
					"internal_error", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
