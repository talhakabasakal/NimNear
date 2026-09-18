package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveIgnoresSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:54321"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 192.0.2.1")
	req.Header.Set("X-Real-IP", "198.51.100.9")

	got := Resolve(req, nil)
	if got != "203.0.113.10" {
		t.Fatalf("untrusted peer client IP = %q, want RemoteAddr", got)
	}
}

func TestResolveWalksTrustedProxyChain(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8", "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:443"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.8, 192.0.2.10")

	got := Resolve(req, trusted)
	if got != "198.51.100.7" {
		t.Fatalf("trusted chain client IP = %q, want original client", got)
	}
}

func TestResolveMalformedForwardedFallsBackSafely(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"127.0.0.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "not-an-ip, also-bad")

	got := Resolve(req, trusted)
	if got != "127.0.0.1" {
		t.Fatalf("malformed XFF fallback = %q, want RemoteAddr", got)
	}
}

func TestResolveIPv6(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"::1/128"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:443"
	req.Header.Set("X-Forwarded-For", "2001:db8::1")

	got := Resolve(req, trusted)
	if got != "2001:db8::1" {
		t.Fatalf("IPv6 client IP = %q", got)
	}
}

func TestResolveUsesXRealIPWhenPeerTrustedAndXFFMissing(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:80"
	req.Header.Set("X-Real-IP", "198.51.100.12")

	got := Resolve(req, trusted)
	if got != "198.51.100.12" {
		t.Fatalf("X-Real-IP = %q", got)
	}
}

func TestParseTrustedProxiesRejectsInvalidValues(t *testing.T) {
	if _, err := ParseTrustedProxies([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("expected invalid CIDR to fail")
	}
}

func TestMiddlewareStoresResolvedIP(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	handler := Middleware(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromRequest(r) != "198.51.100.20" {
			t.Errorf("context IP = %q", FromRequest(r))
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:443"
	req.Header.Set("X-Forwarded-For", "198.51.100.20")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}
