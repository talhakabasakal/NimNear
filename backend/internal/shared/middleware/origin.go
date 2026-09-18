package middleware

import (
	"net/http"
	"net/url"
	"strings"

	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

const defaultSessionCookieName = "nimnear_session"

// CookieMutationOrigin rejects cross-site cookie-authenticated mutations.
// CORS allowlists do not stop simple cross-origin form POSTs that send cookies.
func CookieMutationOrigin(allowedOrigins []string, cookieNames ...string) func(http.Handler) http.Handler {
	cookieName := defaultSessionCookieName
	if len(cookieNames) > 0 && strings.TrimSpace(cookieNames[0]) != "" {
		cookieName = cookieNames[0]
	}
	allowed := allowedOriginSet(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isStateChangingMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if hasExplicitBearer(r) {
				next.ServeHTTP(w, r)
				return
			}
			if !hasSessionCookie(r, cookieName) {
				next.ServeHTTP(w, r)
				return
			}
			if !browserOriginAllowed(r, allowed) {
				response.Error(w, domainErr.NewWithCode(domainErr.ErrForbidden, "request_origin_forbidden", "request origin is not allowed", nil))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isStateChangingMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func hasExplicitBearer(r *http.Request) bool {
	parts := strings.SplitN(strings.TrimSpace(r.Header.Get("Authorization")), " ", 2)
	return len(parts) == 2 && strings.EqualFold(parts[0], "bearer") && strings.TrimSpace(parts[1]) != ""
}

func hasSessionCookie(r *http.Request, cookieName string) bool {
	cookie, err := r.Cookie(cookieName)
	return err == nil && cookie != nil && strings.TrimSpace(cookie.Value) != ""
}

func allowedOriginSet(origins []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" || trimmed == "*" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	return allowed
}

func browserOriginAllowed(r *http.Request, allowed map[string]struct{}) bool {
	origin, present := requestBrowserOrigin(r)
	if !present {
		return false
	}
	_, ok := allowed[origin]
	return ok
}

func requestBrowserOrigin(r *http.Request) (string, bool) {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return origin, true
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return "", false
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", true
	}
	return parsed.Scheme + "://" + parsed.Host, true
}
