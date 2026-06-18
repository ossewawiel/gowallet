// Package httpapi is the transport layer: it builds the chi router, wires the
// oapi-codegen strict-server handlers to the wallet core, owns the shared error
// envelope, and serves the infra routes (/openapi.yaml, /swagger).
//
// It depends only on internal/wallet — never on internal/sqlitestore.
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// Deps are the collaborators the router needs. Only shared, process-wide things:
// the services, the spec bytes, and the JWT signing config. Nothing
// request-specific (that rides in r.Context()).
type Deps struct {
	Health    *wallet.HealthService
	Wallet    *wallet.WalletService
	SpecYAML  []byte
	JWTSecret string
	JWTTTL    time.Duration
}

// publicPaths are the only routes reachable without a Bearer token. Everything
// else is protected by default (mirrors the spec's global `security` + the
// per-operation `security: []` opt-outs). Infra routes (/openapi.yaml,
// /swagger) aren't in the spec's paths, so they're enforced here too.
var publicPaths = map[string]bool{
	"/healthz":      true,
	"/token":        true,
	"/openapi.yaml": true,
	"/swagger":      true,
}

// NewRouter builds the fully-wired HTTP handler: middleware, the generated
// /healthz route, and the infra routes for spec discovery + Swagger UI.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	// request-id first so everything downstream can echo it; recover turns a
	// panic into a clean 500 + error envelope instead of a dropped connection.
	r.Use(middleware.RequestID)
	r.Use(recoverer)

	// Off-spec requests get the shared error envelope. The 405 handler also
	// emits an Allow header (RFC 9110) listing the methods chi matched.
	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)

	// Generated server interface mounted onto the chi router. We implement the
	// plain ServerInterface (not strict) so the health handler can emit the
	// spec's exact byte order: {"status":"ok","db":"up"}.
	//
	// The kin-openapi validator middleware is applied ONLY to these spec routes
	// (bodies/params/additionalProperties/enums validated at the edge) so the
	// infra routes below stay untouched. If the spec fails to load we panic at
	// startup — a broken contract must never boot.
	srv := &server{
		health:    deps.Health,
		wallet:    deps.Wallet,
		jwtSecret: deps.JWTSecret,
		jwtTTL:    deps.JWTTTL,
	}
	var mws []gen.MiddlewareFunc

	// Auth runs first: it verifies the Bearer token and drops the identity into
	// the context for every spec route EXCEPT the public opt-outs. We gate on
	// the request path so /token + /healthz stay open while everything else is
	// protected by default — the same shape as the spec's global `security`.
	if deps.JWTSecret != "" {
		auth := Authenticator(deps.JWTSecret)
		mws = append(mws, func(next http.Handler) http.Handler {
			protected := auth(next)
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if publicPaths[r.URL.Path] {
					next.ServeHTTP(w, r)
					return
				}
				protected.ServeHTTP(w, r)
			})
		})
	}

	if len(deps.SpecYAML) > 0 {
		validate, err := newValidator(deps.SpecYAML)
		if err != nil {
			panic("httpapi: load spec for validation: " + err.Error())
		}
		mws = append(mws, gen.MiddlewareFunc(validate))
	}
	gen.HandlerWithOptions(srv, gen.ChiServerOptions{
		BaseRouter:  r,
		Middlewares: mws,
		// A malformed path param (e.g. a bad %-escape in account_id) is a
		// client error; emit the shared envelope as a documented 400 rather
		// than the generator's default plain-text response.
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed path parameter")
		},
	})

	// Infra routes — not in the spec's paths; serve the live contract + UI.
	r.Get("/openapi.yaml", serveSpec(deps.SpecYAML))
	r.Get("/swagger", serveSwagger)

	return r
}
