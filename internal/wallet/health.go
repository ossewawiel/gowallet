package wallet

import "context"

// Pinger is the wallet core's view of "something that can confirm the
// database is reachable". sqlitestore implements it via *sql.DB.PingContext.
// Defining it here (not in sqlitestore) keeps the dependency arrow pointing
// at the core: httpapi → wallet ← sqlitestore.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health is the result of a readiness check. Status is "ok" or "degraded";
// DB is "up" or "down".
type Health struct {
	Status string
	DB     string
}

// HealthService answers "is the service healthy?" by pinging its Pinger.
// It holds no per-request state — safe to share across requests.
type HealthService struct {
	db Pinger
}

// NewHealthService wires a HealthService to a Pinger.
func NewHealthService(db Pinger) *HealthService {
	return &HealthService{db: db}
}

// Check pings the database and reports health. A nil ping is {ok, up};
// any error is {degraded, down}.
func (s *HealthService) Check(ctx context.Context) Health {
	if err := s.db.Ping(ctx); err != nil {
		return Health{Status: "degraded", DB: "down"}
	}
	return Health{Status: "ok", DB: "up"}
}
