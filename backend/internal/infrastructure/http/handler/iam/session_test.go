package iam

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/masterfabric-go/masterfabric/internal/shared/config"
)

func TestSessionCookieDevelopmentFlags(t *testing.T) {
	h := &Handler{cookie: config.NimiqAuthConfig{CookieName: "nimnear_session", CookieSameSite: "lax"}, sessionTTL: 24 * time.Hour}
	cookie := h.sessionCookie("token")
	if cookie.Name != "nimnear_session" || cookie.Path != "/" || !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 86400 {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
}

func TestSessionCookieProductionFlags(t *testing.T) {
	h := &Handler{cookie: config.NimiqAuthConfig{CookieName: "nimnear_session", CookieSecure: true, CookieSameSite: "none"}, sessionTTL: 24 * time.Hour}
	cookie := h.sessionCookie("token")
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("production cookie must be HttpOnly+Secure+SameSite=None: %#v", cookie)
	}
}

func TestLogoutExpiresSessionCookie(t *testing.T) {
	h := &Handler{cookie: config.NimiqAuthConfig{CookieName: "nimnear_session", CookieSameSite: "lax"}, sessionTTL: 24 * time.Hour}
	recorder := httptest.NewRecorder()
	h.Logout(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	result := recorder.Result()
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d", result.StatusCode)
	}
	cookies := result.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "nimnear_session" || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout cookie=%#v", cookies)
	}
}
