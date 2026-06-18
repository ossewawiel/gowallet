package wallet_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// fakePinger lets us drive HealthService without a real database.
type fakePinger struct {
	err error
}

func (f fakePinger) Ping(_ context.Context) error { return f.err }

func TestHealthService_Check_ReportsUp(t *testing.T) {
	t.Parallel()

	svc := wallet.NewHealthService(fakePinger{err: nil})
	got := svc.Check(context.Background())

	if got.Status != "ok" || got.DB != "up" {
		t.Fatalf("healthy ping: want {ok up}, got {%s %s}", got.Status, got.DB)
	}
}

func TestHealthService_Check_ReportsDown(t *testing.T) {
	t.Parallel()

	svc := wallet.NewHealthService(fakePinger{err: errors.New("connection refused")})
	got := svc.Check(context.Background())

	if got.Status != "degraded" || got.DB != "down" {
		t.Fatalf("failing ping: want {degraded down}, got {%s %s}", got.Status, got.DB)
	}
}
