package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestCookieMutationOriginAllowsConfiguredOrigin(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/payment-requests/x/cancel", map[string]string{"Origin": "https://app.example.com"}, true, false)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginRejectsForeignOrigin(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/payment-requests/x/cancel", map[string]string{"Origin": "https://evil.example"}, true, false)
	assertOriginForbidden(t, recorder)
}

func TestCookieMutationOriginRejectsForeignReferer(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/auth/logout", map[string]string{"Referer": "https://evil.example/attack"}, true, false)
	assertOriginForbidden(t, recorder)
}

func TestCookieMutationOriginAllowsValidRefererFallback(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/events/11111111-1111-1111-1111-111111111111/purchases", map[string]string{"Referer": "https://app.example.com/events/1"}, true, false)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginRejectsMissingOriginAndReferer(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/auth/logout", nil, true, false)
	assertOriginForbidden(t, recorder)
}

func TestCookieMutationOriginAllowsJSONMutationFromAllowedOrigin(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPatch, "/api/v1/me/profile", map[string]string{
		"Origin":       "https://app.example.com",
		"Content-Type": "application/json",
	}, true, false)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginDoesNotTreatCookieAsBearer(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/events/x/purchases", map[string]string{"Origin": "https://evil.example"}, true, false)
	assertOriginForbidden(t, recorder)
}

func TestCookieMutationOriginBearerWithCookieDoesNotUseCookieBypass(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/payment-requests/x/cancel", map[string]string{"Origin": "https://evil.example"}, true, true)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginAllowsBearerWithoutOrigin(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/payment-requests/x/cancel", nil, false, true)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginBearerDoesNotSkipWhenOnlyCookiePresent(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/auth/logout", map[string]string{"Authorization": "Bearer"}, true, false)
	assertOriginForbidden(t, recorder)
}

func TestCookieMutationOriginSkipsSafeMethods(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodGet, "/api/v1/me", map[string]string{"Origin": "https://evil.example"}, true, false)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCookieMutationOriginSkipsUnauthenticatedPublicPosts(t *testing.T) {
	recorder := serveCookieMutation(t, http.MethodPost, "/api/v1/auth/login", nil, false, false)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func serveCookieMutation(t *testing.T, method, path string, headers map[string]string, cookie, bearer bool) *httptest.ResponseRecorder {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if cookie {
		request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-token"})
	}
	if bearer {
		request.Header.Set("Authorization", "Bearer api-token")
	}
	recorder := httptest.NewRecorder()
	CookieMutationOrigin([]string{"https://app.example.com"})(next).ServeHTTP(recorder, request)
	return recorder
}

func assertOriginForbidden(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload domainErr.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if payload.ErrorCode != "request_origin_forbidden" {
		t.Fatalf("error_code=%q body=%s", payload.ErrorCode, recorder.Body.String())
	}
	if payload.Message == "https://app.example.com" || payload.Message == `["https://app.example.com"]` {
		t.Fatal("response exposed the origin allowlist")
	}
}
