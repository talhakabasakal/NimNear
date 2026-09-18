package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("nosniff = %q", recorder.Header().Get("X-Content-Type-Options"))
	}
	if recorder.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatalf("referrer = %q", recorder.Header().Get("Referrer-Policy"))
	}
	if recorder.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("frame = %q", recorder.Header().Get("X-Frame-Options"))
	}
	if recorder.Header().Get("Permissions-Policy") == "" {
		t.Fatal("expected Permissions-Policy")
	}
}
