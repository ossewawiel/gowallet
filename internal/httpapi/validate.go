package httpapi

import (
	"context"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	oapimw "github.com/oapi-codegen/nethttp-middleware"
)

// newValidator builds the kin-openapi request-validation middleware from the
// live spec bytes. It validates every request it wraps against api/openapi.yaml
// at the edge (bodies, params, additionalProperties, enums) so handlers can
// assume valid input — the spec-first discipline from docs/REST_API_GUIDELINES.md.
//
// It is mounted ONLY on the generated (spec-defined) routes, so infra routes
// (/openapi.yaml, /swagger) are never seen by it. Validation failures are
// funnelled through the shared error envelope as a 400 invalid_input.
func newValidator(specYAML []byte) (func(http.Handler) http.Handler, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specYAML)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, err
	}

	opts := &oapimw.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: func(_ context.Context, _ error, w http.ResponseWriter, r *http.Request, _ oapimw.ErrorHandlerOpts) {
			writeError(w, r, http.StatusBadRequest, "invalid_input", "request does not satisfy the API contract")
		},
	}
	return oapimw.OapiRequestValidatorWithOptions(doc, opts), nil
}
