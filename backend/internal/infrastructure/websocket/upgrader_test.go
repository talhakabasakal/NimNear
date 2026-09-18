package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginCheckerRejectsEmptyOriginWhenDisallowed(t *testing.T) {
	check := originChecker([]string{"https://app.example.com"}, false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	if check(request) {
		t.Fatal("empty origin must be rejected in production")
	}
}

func TestOriginCheckerAllowsEmptyOriginWhenConfigured(t *testing.T) {
	check := originChecker([]string{"https://app.example.com"}, true)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	if !check(request) {
		t.Fatal("empty origin should be allowed for non-browser clients")
	}
}

func TestOriginCheckerAllowsConfiguredOrigin(t *testing.T) {
	check := originChecker([]string{"https://app.example.com"}, false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.Header.Set("Origin", "https://app.example.com")
	if !check(request) {
		t.Fatal("configured origin should be allowed")
	}
}

func TestOriginCheckerRejectsForeignOrigin(t *testing.T) {
	check := originChecker([]string{"https://app.example.com"}, false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.Header.Set("Origin", "https://evil.example")
	if check(request) {
		t.Fatal("foreign origin must be rejected")
	}
}

func TestOriginCheckerEmptyAllowlistRejectsBrowserOriginWhenEmptyDisallowed(t *testing.T) {
	check := originChecker(nil, false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.Header.Set("Origin", "https://app.example.com")
	if check(request) {
		t.Fatal("browser origin must not be allowed without an allowlist in production")
	}
}
