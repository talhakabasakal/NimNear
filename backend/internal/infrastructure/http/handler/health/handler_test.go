package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type failingPinger struct{}

func (failingPinger) Ping(context.Context) error {
	return errors.New("connection refused host=secret-db:5432")
}

func TestReadiness_DoesNotExposeInternalErrors(t *testing.T) {
	handler := &Handler{db: failingPinger{}}

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret-db")
	assert.Contains(t, rec.Body.String(), "unhealthy")
}

func TestReadiness_ReportsNimiqRPCWithoutFailingCoreReady(t *testing.T) {
	handler := NewHandler(nil, nil).WithNimiqRPC(true, failingRPC{})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"nimiq_rpc":"unhealthy"`)
	assert.NotContains(t, rec.Body.String(), "secret-rpc")
}

func TestReadiness_MarksNimiqRPCDisabledWhenPaymentsOff(t *testing.T) {
	handler := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler.Readiness(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"nimiq_rpc":"disabled"`)
}

func TestPayments_ReportsNotConfiguredWithoutSecrets(t *testing.T) {
	handler := NewHandler(nil, nil).WithPaymentStatus("", false, false, true)
	req := httptest.NewRequest(http.MethodGet, "/health/payments", nil)
	rec := httptest.NewRecorder()
	handler.Payments(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"status":"not_configured"`)
	assert.Contains(t, body, `"payment_configured":false`)
	assert.Contains(t, body, `"rpc_configured":false`)
	assert.Contains(t, body, `"merchant_address_configured":false`)
	assert.Contains(t, body, `"websocket_enabled":true`)
	assert.Contains(t, body, `"nimiq_rpc":"disabled"`)
	assert.NotContains(t, body, "NQ")
	assert.NotContains(t, body, "rpc.")
	assert.NotContains(t, body, "secret")
	assert.NotContains(t, body, "token")
}

func TestPayments_ReportsReadyWithCanonicalMainnetNetwork(t *testing.T) {
	handler := NewHandler(nil, nil).
		WithNimiqRPC(true, healthyRPC{}).
		WithPaymentStatus("main-albatross", true, true, true)
	req := httptest.NewRequest(http.MethodGet, "/health/payments", nil)
	rec := httptest.NewRecorder()
	handler.Payments(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"status":"ready"`)
	assert.Contains(t, body, `"payment_configured":true`)
	assert.Contains(t, body, `"network":"MainAlbatross"`)
	assert.Contains(t, body, `"rpc_configured":true`)
	assert.Contains(t, body, `"merchant_address_configured":true`)
	assert.Contains(t, body, `"nimiq_rpc":"healthy"`)
	assert.NotContains(t, body, "test-albatross")
	assert.NotContains(t, body, "TestAlbatross")
}

func TestPayments_FailsWhenConfiguredRPCIsUnhealthy(t *testing.T) {
	handler := NewHandler(nil, nil).
		WithNimiqRPC(true, failingRPC{}).
		WithPaymentStatus("MainAlbatross", true, true, false)
	req := httptest.NewRequest(http.MethodGet, "/health/payments", nil)
	rec := httptest.NewRecorder()
	handler.Payments(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"status":"not_ready"`)
	assert.Contains(t, body, `"nimiq_rpc":"unhealthy"`)
	assert.Contains(t, body, `"websocket_enabled":false`)
	assert.NotContains(t, body, "secret-rpc")
}

type failingRPC struct{}

func (failingRPC) Check(context.Context) error {
	return errors.New("connection refused host=secret-rpc:8648")
}

type healthyRPC struct{}

func (healthyRPC) Check(context.Context) error {
	return nil
}
