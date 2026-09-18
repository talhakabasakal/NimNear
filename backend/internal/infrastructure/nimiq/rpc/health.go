package rpc

import (
	"context"
	"sync"
	"time"
)

const defaultHealthCacheTTL = 20 * time.Second
const defaultHealthTimeout = 3 * time.Second

// Health reports whether the configured Nimiq RPC answered a read-only probe.
type Health struct {
	client *Client
	ttl    time.Duration

	mu        sync.Mutex
	checkedAt time.Time
	err       error
}

// NewHealth caches getBatchNumber probes so readiness checks do not hammer RPC.
func NewHealth(client *Client) *Health {
	if client == nil {
		return nil
	}
	return &Health{client: client, ttl: defaultHealthCacheTTL}
}

// Check runs a cached getBatchNumber probe. It never broadcasts a transaction.
func (h *Health) Check(ctx context.Context) error {
	if h == nil || h.client == nil {
		return ErrRPCUnavailable
	}
	now := time.Now()
	h.mu.Lock()
	if !h.checkedAt.IsZero() && now.Sub(h.checkedAt) < h.ttl {
		err := h.err
		h.mu.Unlock()
		return err
	}
	h.mu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, defaultHealthTimeout)
	defer cancel()
	_, err := h.client.GetBatchNumber(probeCtx)

	h.mu.Lock()
	h.checkedAt = time.Now()
	h.err = err
	h.mu.Unlock()
	return err
}
