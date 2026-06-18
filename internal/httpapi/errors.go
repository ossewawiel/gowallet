package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
)

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
				writeError(w, r, http.StatusInternalServerError,
					"internal_error", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
