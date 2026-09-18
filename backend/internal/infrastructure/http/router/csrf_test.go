package router

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	eventpurchaseHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventpurchase"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	paymentrequestHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/paymentrequest"
	profileHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/profile"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type csrfAuthStub struct{}

func (csrfAuthStub) HashPassword(string) (string, error) { return "", nil }
func (csrfAuthStub) VerifyPassword(string, string) error { return nil }
func (csrfAuthStub) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "", nil
}
func (csrfAuthStub) ValidateToken(context.Context, string) (*service.TokenClaims, error) {
	return &service.TokenClaims{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}, nil
}

func csrfRouter() http.Handler {
	deps := fullyWiredDeps()
	deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	deps.CORSAllowedOrigins = []string{"https://app.example.com"}
	deps.SessionCookieName = "nimnear_session"
	deps.AuthService = csrfAuthStub{}
	deps.GatewayPipeline = nil
	deps.IAMHandler = iamHandler.NewHandler(nil, nil, nil, nil, nil, config.NimiqAuthConfig{CookieName: "nimnear_session", CookieSameSite: "lax"}, 0, nil)
	deps.PurchaseHandler = &eventpurchaseHandler.Handler{}
	deps.PaymentRequestHandler = &paymentrequestHandler.Handler{}
	deps.ProfileHandler = &profileHandler.Handler{}
	return New(deps)
}

func TestCookieMutationsRejectForeignOrigin(t *testing.T) {
	handler := csrfRouter()
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "logout", method: http.MethodPost, path: "/api/v1/auth/logout"},
		{name: "payment request cancel", method: http.MethodPost, path: "/api/v1/payment-requests/" + uuid.New().String() + "/cancel"},
		{name: "event purchase create", method: http.MethodPost, path: "/api/v1/events/" + uuid.New().String() + "/purchases"},
		{name: "json profile mutation", method: http.MethodPatch, path: "/api/v1/me/profile"},
		{name: "delete account", method: http.MethodDelete, path: "/api/v1/me"},
		{name: "calendar archive", method: http.MethodPost, path: "/api/v1/calendars/" + uuid.New().String() + "/archive"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Origin", "https://evil.example")
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			assertCSRFForbidden(t, recorder)
		})
	}
}

func TestCookieMutationsAllowConfiguredOrigin(t *testing.T) {
	handler := csrfRouter()
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "logout", method: http.MethodPost, path: "/api/v1/auth/logout"},
		{name: "payment request cancel", method: http.MethodPost, path: "/api/v1/payment-requests/" + uuid.New().String() + "/cancel"},
		{name: "event purchase create", method: http.MethodPost, path: "/api/v1/events/" + uuid.New().String() + "/purchases"},
		{name: "json profile mutation", method: http.MethodPatch, path: "/api/v1/me/profile"},
		{name: "delete account", method: http.MethodDelete, path: "/api/v1/me"},
		{name: "calendar archive", method: http.MethodPost, path: "/api/v1/calendars/" + uuid.New().String() + "/archive"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusForbidden {
				t.Fatalf("allowed origin was rejected: %s", recorder.Body.String())
			}
		})
	}
}

func TestCookieMutationsRefererFallback(t *testing.T) {
	handler := csrfRouter()
	allowed := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	allowed.Header.Set("Referer", "https://app.example.com/wallet")
	allowed.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
	allowedRec := httptest.NewRecorder()
	handler.ServeHTTP(allowedRec, allowed)
	if allowedRec.Code == http.StatusForbidden {
		t.Fatalf("valid referer rejected: %s", allowedRec.Body.String())
	}

	foreign := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	foreign.Header.Set("Referer", "https://evil.example/")
	foreign.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
	foreignRec := httptest.NewRecorder()
	handler.ServeHTTP(foreignRec, foreign)
	assertCSRFForbidden(t, foreignRec)
}

func TestCookieMutationsRejectMissingOriginAndReferer(t *testing.T) {
	handler := csrfRouter()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	assertCSRFForbidden(t, recorder)
}

func TestBearerMutationsContinueWithoutBrowserOrigin(t *testing.T) {
	handler := csrfRouter()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests/"+uuid.New().String()+"/cancel", nil)
	request.Header.Set("Authorization", "Bearer api-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusForbidden {
		t.Fatalf("bearer client was origin-blocked: %s", recorder.Body.String())
	}
}

func TestWebSocketGetIsNotOriginBlockedAsHTTPMutation(t *testing.T) {
	handler := csrfRouter()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.Header.Set("Origin", "https://evil.example")
	request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusForbidden && strings.Contains(recorder.Body.String(), "request_origin_forbidden") {
		t.Fatalf("websocket GET was treated as an HTTP mutation: %s", recorder.Body.String())
	}
}

func assertCSRFForbidden(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload domainErr.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if payload.ErrorCode != "request_origin_forbidden" {
		t.Fatalf("error_code=%q", payload.ErrorCode)
	}
}
