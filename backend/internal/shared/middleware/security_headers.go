package middleware

import "net/http"

// SecurityHeaders sets conservative browser defaults that do not conflict with
// Nimiq Mini App or Hub popup flows. Content-Security-Policy is deferred.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("X-Frame-Options", "SAMEORIGIN")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")
		next.ServeHTTP(w, r)
	})
}
