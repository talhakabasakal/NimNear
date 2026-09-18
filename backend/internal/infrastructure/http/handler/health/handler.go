// Package health provides liveness and readiness HTTP probes.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/shared/nimiq"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/redis/go-redis/v9"
)

// Handler provides health check endpoints.
type Handler struct {
	db                        dbPinger
	redis                     redisPinger
	paymentsEnabled           bool
	nimiqRPC                  RPCChecker
	paymentNetwork            string
	rpcConfigured             bool
	merchantAddressConfigured bool
	websocketEnabled          bool
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

// WithPaymentStatus attaches non-secret payment readiness flags. It never stores
// the merchant address value, RPC URL, credentials, or private keys.
func (h *Handler) WithPaymentStatus(network string, merchantConfigured, rpcConfigured, websocketEnabled bool) *Handler {
	if h != nil {
		h.paymentNetwork = strings.TrimSpace(network)
		h.merchantAddressConfigured = merchantConfigured
		h.rpcConfigured = rpcConfigured
		h.websocketEnabled = websocketEnabled
	}
	return h
}

// HealthResponse is the JSON structure for health checks.
type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

// PaymentReadinessResponse is a public, non-secret payment configuration snapshot.
type PaymentReadinessResponse struct {
	Status                    string            `json:"status"`
	PaymentConfigured         bool              `json:"payment_configured"`
	Network                   string            `json:"network"`
	RPCConfigured             bool              `json:"rpc_configured"`
	MerchantAddressConfigured bool              `json:"merchant_address_configured"`
	WebSocketEnabled          bool              `json:"websocket_enabled"`
	Services                  map[string]string `json:"services"`
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
	services, coreHealthy, _ := h.probeServices(r.Context())

	status := "ready"
	code := http.StatusOK
	if !coreHealthy {
		status = "not ready"
		code = http.StatusServiceUnavailable
	}

	response.JSON(w, code, HealthResponse{
		Status:   status,
		Services: services,
	})
}

// Payments reports whether Mainnet payment verification is configured and
// currently reachable. It exposes only booleans, the canonical consensus
// network name, and the same non-secret service statuses as /health/ready.
func (h *Handler) Payments(w http.ResponseWriter, r *http.Request) {
	services, coreHealthy, rpcHealthy := h.probeServices(r.Context())
	configured := h.paymentsEnabled && h.rpcConfigured && h.merchantAddressConfigured && strings.TrimSpace(h.paymentNetwork) != ""

	status := "not_configured"
	code := http.StatusOK
	if !coreHealthy || (configured && !rpcHealthy) {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	} else if configured && rpcHealthy {
		status = "ready"
	}

	response.JSON(w, code, PaymentReadinessResponse{
		Status:                    status,
		PaymentConfigured:         configured,
		Network:                   canonicalPaymentNetwork(h.paymentNetwork),
		RPCConfigured:             h.rpcConfigured,
		MerchantAddressConfigured: h.merchantAddressConfigured,
		WebSocketEnabled:          h.websocketEnabled,
		Services:                  services,
	})
}

func (h *Handler) probeServices(ctx context.Context) (map[string]string, bool, bool) {
	services := make(map[string]string)
	coreHealthy := true
	rpcHealthy := false

	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			slog.Error("readiness check failed", "service", "postgres", "error", err)
			services["postgres"] = "unhealthy"
			coreHealthy = false
		} else {
			services["postgres"] = "healthy"
		}
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			slog.Error("readiness check failed", "service", "redis", "error", err)
			services["redis"] = "unhealthy"
			coreHealthy = false
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
			rpcHealthy = true
		}
	} else {
		services["nimiq_rpc"] = "disabled"
	}

	return services, coreHealthy, rpcHealthy
}

func canonicalPaymentNetwork(value string) string {
	parsed, err := nimiq.ParseNetwork(value)
	if err != nil {
		return ""
	}
	return parsed.ConsensusName
}
