package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	iammodel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
)

type cookieAuthStub struct{ token string }

func (cookieAuthStub) HashPassword(string) (string, error) { return "", nil }
func (cookieAuthStub) VerifyPassword(string, string) error { return nil }
func (cookieAuthStub) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "", nil
}
func (s cookieAuthStub) ValidateToken(_ context.Context, token string) (*service.TokenClaims, error) {
	s.token = token
	return &service.TokenClaims{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}, nil
}

func TestJWTAuthAcceptsSessionCookie(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserIDFromContext(r.Context()); !ok {
			t.Fatal("user id missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: "custom_session", Value: "cookie-token"})
	response := httptest.NewRecorder()
	JWTAuth(cookieAuthStub{}, "custom_session")(next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

type statusLookupStub struct {
	user *iammodel.User
	err  error
}

func (s statusLookupStub) Create(context.Context, *iammodel.User) error { return nil }
func (s statusLookupStub) GetByID(context.Context, uuid.UUID) (*iammodel.User, error) {
	return s.user, s.err
}
func (s statusLookupStub) GetByEmail(context.Context, string) (*iammodel.User, error) {
	return nil, nil
}
func (s statusLookupStub) Update(context.Context, *iammodel.User) error { return nil }
func (s statusLookupStub) Delete(context.Context, uuid.UUID) error      { return nil }
func (s statusLookupStub) List(context.Context, int, int) ([]*iammodel.User, int, error) {
	return nil, 0, nil
}

func TestRequireActiveAccountRejectsDeletedUser(t *testing.T) {
	deletedAt := time.Now().UTC()
	lookup := statusLookupStub{user: &iammodel.User{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Status: iammodel.UserStatusInactive, DeletedAt: &deletedAt}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request = request.WithContext(context.WithValue(request.Context(), ContextKeyUserID, lookup.user.ID))
	response := httptest.NewRecorder()
	RequireActiveAccount(lookup)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("deleted user reached handler")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRequireActiveAccountAllowsActiveUser(t *testing.T) {
	lookup := statusLookupStub{user: &iammodel.User{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Status: iammodel.UserStatusActive}}
	called := false
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request = request.WithContext(context.WithValue(request.Context(), ContextKeyUserID, lookup.user.ID))
	response := httptest.NewRecorder()
	RequireActiveAccount(lookup)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)
	if !called || response.Code != http.StatusNoContent {
		t.Fatalf("active user rejected status=%d", response.Code)
	}
}

func TestJWTAuthPrefersBearerOverCookie(t *testing.T) {
	seen := ""
	auth := authServiceFunc(func(token string) (*service.TokenClaims, error) {
		seen = token
		return &service.TokenClaims{UserID: uuid.New()}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer header-token")
	request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "cookie-token"})
	response := httptest.NewRecorder()
	JWTAuth(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
	if seen != "header-token" {
		t.Fatalf("validated %q", seen)
	}
}

type authServiceFunc func(string) (*service.TokenClaims, error)

func (authServiceFunc) HashPassword(string) (string, error) { return "", nil }
func (authServiceFunc) VerifyPassword(string, string) error { return nil }
func (authServiceFunc) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "", nil
}
func (f authServiceFunc) ValidateToken(_ context.Context, token string) (*service.TokenClaims, error) {
	return f(token)
}
