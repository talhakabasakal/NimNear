package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type contextKey struct{}

// TrustedProxies is an explicit allowlist of immediate peers that may supply
// forwarded client addresses. Direct clients that are not on this list never
// have X-Forwarded-For or X-Real-IP honored.
type TrustedProxies []netip.Prefix

// Middleware resolves the client IP once per request and stores it on the context.
func Middleware(trusted TrustedProxies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := Resolve(r, trusted)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, ip)))
		})
	}
}

// FromRequest returns the client IP stored by Middleware, or resolves against
// an empty trusted-proxy set when middleware did not run.
func FromRequest(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if ip, ok := r.Context().Value(contextKey{}).(string); ok && ip != "" {
		return ip
	}
	return Resolve(r, nil)
}

// Resolve determines the client IP using an explicit trusted-proxy allowlist.
//
// If the immediate RemoteAddr is not a trusted proxy, forwarded headers are
// ignored. If it is trusted, the X-Forwarded-For chain is walked from the
// right, skipping trusted hops, and the first untrusted valid address is used.
// Malformed entries are skipped. X-Real-IP is used only when the peer is
// trusted and X-Forwarded-For did not yield a usable address.
func Resolve(r *http.Request, trusted TrustedProxies) string {
	if r == nil {
		return "unknown"
	}
	remote, ok := parseIP(r.RemoteAddr)
	if !ok {
		if strings.TrimSpace(r.RemoteAddr) == "" {
			return "unknown"
		}
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && strings.TrimSpace(host) != "" {
			return host
		}
		return r.RemoteAddr
	}
	if !trusted.contains(remote) {
		return formatIP(remote)
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if ip, found := clientFromForwarded(forwarded, trusted); found {
			return formatIP(ip)
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if ip, ok := parseIP(realIP); ok {
			return formatIP(ip)
		}
	}
	return formatIP(remote)
}

func clientFromForwarded(header string, trusted TrustedProxies) (netip.Addr, bool) {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip, ok := parseIP(parts[i])
		if !ok {
			continue
		}
		if trusted.contains(ip) {
			continue
		}
		return ip, true
	}
	for _, part := range parts {
		ip, ok := parseIP(part)
		if ok {
			return ip, true
		}
	}
	return netip.Addr{}, false
}

func parseIP(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, false
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func formatIP(addr netip.Addr) string {
	return addr.Unmap().String()
}

func (t TrustedProxies) contains(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range t {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies parses explicit IPv4/IPv6 addresses or CIDR prefixes.
// Empty input is valid and means forwarded headers are never trusted.
func ParseTrustedProxies(values []string) (TrustedProxies, error) {
	if len(values) == 0 {
		return nil, nil
	}
	parsed := make(TrustedProxies, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		prefix, err := parseProxyPrefix(value)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, prefix)
	}
	return parsed, nil
}

func parseProxyPrefix(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, err
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	bits := addr.BitLen()
	return netip.PrefixFrom(addr, bits), nil
}
