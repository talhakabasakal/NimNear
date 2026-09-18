package middleware

import (
	"net/http"
	"strings"
)

// ExtractBearerToken returns a JWT from the Authorization header only.
// Query-string tokens are rejected because they leak through logs, history,
// proxies, and observability systems. Browser WebSocket clients authenticate
// with the HttpOnly session cookie; non-browser clients use Authorization.
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
