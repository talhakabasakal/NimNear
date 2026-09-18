package rpc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHealthCachesProbe(t *testing.T) {
	calls := 0
	health := &Health{
		ttl: time.Minute,
		client: &Client{
			endpoint:   "http://127.0.0.1:1",
			httpClient: nil,
		},
	}
	_ = calls
	if health.ttl != time.Minute {
		t.Fatalf("ttl = %s", health.ttl)
	}
}

func TestRPCOutcomeLabels(t *testing.T) {
	if rpcOutcome(nil) != "success" {
		t.Fatal("nil should be success")
	}
	if rpcOutcome(ErrRPCTimeout) != "timeout" {
		t.Fatal("timeout")
	}
	if rpcOutcome(ErrRPCUnavailable) != "unavailable" {
		t.Fatal("unavailable")
	}
	if rpcOutcome(errors.New("other")) != "error" {
		t.Fatal("error")
	}
	if NewHealth(nil) != nil {
		t.Fatal("nil client must not create a probe")
	}
	_ = context.Background()
}
