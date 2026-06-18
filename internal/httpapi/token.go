package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// IssueToken handles POST /token: it mints a signed HS256 JWT for the supplied
// account_id + role. This is a demo token mint, not a credential login — there
// is no password store in scope (documented in SOLUTION.md). An unknown role is
// a 422; a malformed body is a 400.
func (s *server) IssueToken(w http.ResponseWriter, r *http.Request) {
	var body gen.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}

	role, err := wallet.ParseRole(string(body.Role))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_role", "role must be 'member' or 'admin'")
		return
	}
	if body.AccountId == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_input", "account_id is required")
		return
	}

	token, err := IssueToken(s.jwtSecret, s.jwtTTL, wallet.Identity{AccountID: body.AccountId, Role: role})
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
