package httpapi_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// fakePinger drives the health seam without a real database.
type fakePinger struct {
	err error
}

func (f fakePinger) Ping(_ context.Context) error { return f.err }

func newServer(t *testing.T, pinger wallet.Pinger) http.Handler {
	t.Helper()
	svc := wallet.NewHealthService(pinger)
	// A valid minimal spec: the validator must load it at startup. /healthz is
	// covered by the contract; we keep it here so the route is validated too.
	spec := []byte("openapi: 3.0.3\n" +
		"info: { title: t, version: '0' }\n" +
		"paths:\n" +
		"  /healthz:\n" +
		"    get:\n" +
		"      responses:\n" +
		"        '200': { description: ok }\n" +
		"        '503': { description: down }\n")
	return httpapi.NewRouter(httpapi.Deps{Health: svc, SpecYAML: spec})
}

func TestHealthz_200_WhenDBUp(t *testing.T) {
	t.Parallel()

	srv := newServer(t, fakePinger{err: nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != `{"status":"ok","db":"up"}` {
		t.Fatalf("body: want {\"status\":\"ok\",\"db\":\"up\"}, got %q", body)
	}
}

func TestHealthz_503_WhenDBDown(t *testing.T) {
	t.Parallel()

	srv := newServer(t, fakePinger{err: errors.New("db down")})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: want 503, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != `{"status":"degraded","db":"down"}` {
		t.Fatalf("body: want {\"status\":\"degraded\",\"db\":\"down\"}, got %q", body)
	}
}

func TestHealthz_405_HasAllowHeader(t *testing.T) {
	t.Parallel()

	srv := newServer(t, fakePinger{err: nil})
	rec := httptest.NewRecorder()
	// QUERY is a valid HTTP method but not allowed on /healthz.
	req := httptest.NewRequest("QUERY", "/healthz", nil)

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: want 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Fatalf("405 must carry an Allow header (RFC 9110), got none")
	}
}

// guard against accidental import of sqlitestore — kept as a compile-time note.
var _ = io.Discard
