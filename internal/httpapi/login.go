package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// Login handles POST /login: it verifies an account_id + secret against the
// stored credential and, on success, mints a signed HS256 JWT whose role is the
// STORED account's role (never the request). A wrong secret and an unknown
// account return the IDENTICAL 401 envelope so accounts can't be enumerated.
// The secret is never logged.
func (s *server) Login(w http.ResponseWriter, r *http.Request) {
	var body gen.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	// secret is required by the spec (enforced at the kin-openapi edge); guard
	// defensively so a nil pointer can't panic the handler.
	var secret string
	if body.Secret != nil {
		secret = *body.Secret
	}

	id, err := s.wallet.Login(r.Context(), body.AccountId, secret)
	if err != nil {
		// Mapped EXPLICITLY here (not via writeDomainError) so a credential 401
		// can never leak through the shared error mapper, and so the body shape
		// is identical for wrong-secret and unknown-account.
		if errors.Is(err, wallet.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "could not process login")
		return
	}

	token, err := IssueToken(s.jwtSecret, s.jwtTTL, id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, gen.TokenResponse{
		Token:     token,
		TokenType: gen.Bearer,
		ExpiresIn: int(s.jwtTTL / time.Second),
	})
}
