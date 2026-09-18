package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoginResponseOmitsTokenFromJSON(t *testing.T) {
	body, err := json.Marshal(LoginResponse{Token: "super-secret-jwt", User: UserInfo{Email: "user@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "token") || strings.Contains(string(body), "super-secret-jwt") {
		t.Fatalf("browser login JSON leaked JWT: %s", body)
	}
	if !strings.Contains(string(body), `"user"`) {
		t.Fatalf("missing user: %s", body)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  User@Example.COM "); got != "user@example.com" {
		t.Fatalf("NormalizeEmail = %q", got)
	}
}

func TestTokenResponseIncludesTokenForAPIClients(t *testing.T) {
	body, err := json.Marshal(TokenResponse{Token: "api-jwt", User: UserInfo{Email: "user@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"token":"api-jwt"`) {
		t.Fatalf("token response = %s", body)
	}
}
