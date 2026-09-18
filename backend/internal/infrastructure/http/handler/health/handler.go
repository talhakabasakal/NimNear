// Package health provides liveness and readiness HTTP probes.
package health

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/redis/go-redis/v9"
)

// Handler provides health check endpoints.
type Handler struct {
	db              dbPinger
	redis           redisPinger
	paymentsEnabled bool
	nimiqRPC        RPCChecker
}

type dbPinger interface {
	Ping(ctx context.Context) error
}

type redisPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// RPCChecker is a cached, read-only Nimiq RPC probe. It must never send transactions.
type RPCChecker interface {
	Check(ctx context.Context) error
}

// NewHandler creates a new health handler.
func NewHandler(db *pgxpool.Pool, redis *redis.Client) *Handler {
	h := &Handler{}
	if db != nil {
		h.db = db
	}
	if redis != nil {
		h.redis = redis
	}
	return h
}

// WithNimiqRPC attaches optional payment RPC readiness without changing core probes.
func (h *Handler) WithNimiqRPC(enabled bool, checker RPCChecker) *Handler {
	if h != nil {
		h.paymentsEnabled = enabled
		h.nimiqRPC = checker
	}
	return h
}

// HealthResponse is the JSON structure for health checks.
type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

// Liveness returns 200 if the server is alive.
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// Readiness checks PostgreSQL and Redis. Nimiq RPC is reported when payments are
// enabled, but an RPC outage does not fail this endpoint: payments already fail
// closed, and taking the whole process out of rotation would block free-event
// and auth traffic. Operators must alert on services.nimiq_rpc separately.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services := make(map[string]string)
	healthy := true

	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			slog.Error("readiness check failed", "service", "postgres", "error", err)
			services["postgres"] = "unhealthy"
			healthy = false
		} else {
			services["postgres"] = "healthy"
		}
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			slog.Error("readiness check failed", "service", "redis", "error", err)
			services["redis"] = "unhealthy"
			healthy = false
		} else {
			services["redis"] = "healthy"
		}
	}

	if h.paymentsEnabled {
		if h.nimiqRPC == nil {
			services["nimiq_rpc"] = "unhealthy"
			slog.Error("readiness check failed", "service", "nimiq_rpc", "error", "probe not configured")
		} else if err := h.nimiqRPC.Check(ctx); err != nil {
			slog.Error("readiness check failed", "service", "nimiq_rpc", "error", err)
			services["nimiq_rpc"] = "unhealthy"
		} else {
			services["nimiq_rpc"] = "healthy"
		}
	} else {
		services["nimiq_rpc"] = "disabled"
	}

	status := "ready"
	code := http.StatusOK
	if !healthy {
		status = "not ready"
		code = http.StatusServiceUnavailable
	}

	response.JSON(w, code, HealthResponse{
		Status:   status,
		Services: services,
	})
}
