package iam

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
)

type handlerAuthRepo struct {
	challenge *usecase.NimiqChallenge
	user      *model.User
}

func (r *handlerAuthRepo) CreateChallenge(_ context.Context, c *usecase.NimiqChallenge) error {
	r.challenge = c
	return nil
}
func (r *handlerAuthRepo) GetChallenge(_ context.Context, _ uuid.UUID) (*usecase.NimiqChallenge, error) {
	return r.challenge, nil
}
func (r *handlerAuthRepo) ConsumeAndResolveIdentity(_ context.Context, _ uuid.UUID, _ []byte, _ time.Time, _ string) (*model.User, error) {
	return r.user, nil
}
func (r *handlerAuthRepo) AddressForUser(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
func (r *handlerAuthRepo) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}

type handlerAuthService struct{}

func (handlerAuthService) HashPassword(string) (string, error) { return "", nil }
func (handlerAuthService) VerifyPassword(string, string) error { return nil }
func (handlerAuthService) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "signed-session", nil
}
func (handlerAuthService) ValidateToken(context.Context, string) (*service.TokenClaims, error) {
	return &service.TokenClaims{UserID: uuid.New()}, nil
}

func testNimiqHandler(t *testing.T, limiter ratelimit.Limiter, cookie config.NimiqAuthConfig) (*Handler, *handlerAuthRepo) {
	t.Helper()
	if cookie.Network == "" {
		cookie = config.NimiqAuthConfig{
			Network: "test-albatross", Environment: "testnet", Domain: "test.nimnear.local",
			ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax",
			ChallengeIPLimit: 1, VerifyIPLimit: 1, AbuseLimit: 1, RateLimitWindow: time.Minute,
		}
	}
	repo := &handlerAuthRepo{user: &model.User{ID: uuid.New(), Status: model.UserStatusActive}}
	return NewHandler(nil, nil, nil, nil, usecase.NewNimiqAuthUseCase(repo, handlerAuthService{}, cookie), cookie, time.Hour, limiter), repo
}

func TestCreateNimiqChallengeRateLimited(t *testing.T) {
	handler, _ := testNimiqHandler(t, ratelimit.NewMemoryLimiter(), config.NimiqAuthConfig{})
	body := `{"wallet_address":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","network":"test-albatross","environment":"testnet","purpose":"AUTH_LOGIN","transport":"mini-app"}`
	first := httptest.NewRecorder()
	handler.CreateNimiqChallenge(first, httptest.NewRequest(http.MethodPost, "/api/v1/auth/nimiq/challenges", strings.NewReader(body)))
	if first.Code != http.StatusCreated {
		t.Fatalf("first challenge status=%d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	handler.CreateNimiqChallenge(second, httptest.NewRequest(http.MethodPost, "/api/v1/auth/nimiq/challenges", strings.NewReader(body)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second challenge status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "authentication_rate_limited") {
		t.Fatalf("expected authentication_rate_limited envelope, got %s", second.Body.String())
	}
}

func TestVerifyNimiqChallengeSetsHttpOnlyCookieAndRateLimitsAbuse(t *testing.T) {
	cookieCfg := config.NimiqAuthConfig{
		Network: "test-albatross", Environment: "testnet", Domain: "test.nimnear.local",
		ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSecure: true, CookieSameSite: "none",
		ChallengeIPLimit: 10, VerifyIPLimit: 1, AbuseLimit: 1, RateLimitWindow: time.Minute,
	}
	handler, repo := testNimiqHandler(t, ratelimit.NewMemoryLimiter(), cookieCfg)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	message := "NIMNear canonical ASCII challenge"
	address := usecase.AddressFromPublicKey(public)
	preimage, err := usecase.SignedBytes(usecase.NimiqTransportMiniApp, message)
	if err != nil {
		t.Fatal(err)
	}
	repo.challenge = &usecase.NimiqChallenge{
		ID: uuid.New(), Purpose: usecase.NimiqAuthPurpose, Transport: usecase.NimiqTransportMiniApp,
		Environment: "testnet", Network: "test-albatross", Domain: "test.nimnear.local", Audience: usecase.NimiqAuthAudience,
		ClaimedAddress: address, Message: message, IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	payload, _ := json.Marshal(map[string]string{
		"challenge_id": repo.challenge.ID.String(),
		"message":      message,
		"public_key":   hex.EncodeToString(public),
		"signature":    hex.EncodeToString(ed25519.Sign(private, preimage)),
	})
	first := httptest.NewRecorder()
	handler.VerifyNimiqChallenge(first, httptest.NewRequest(http.MethodPost, "/api/v1/auth/nimiq/verify", bytes.NewReader(payload)))
	if first.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", first.Code, first.Body.String())
	}
	cookies := first.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "nimnear_session" || cookies[0].Value != "signed-session" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteNoneMode {
		t.Fatalf("session cookie=%#v", cookies)
	}
	if strings.Contains(first.Body.String(), `"token"`) || strings.Contains(first.Body.String(), "signed-session") {
		t.Fatalf("browser verify JSON leaked JWT: %s", first.Body.String())
	}

	second := httptest.NewRecorder()
	handler.VerifyNimiqChallenge(second, httptest.NewRequest(http.MethodPost, "/api/v1/auth/nimiq/verify", bytes.NewReader(payload)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("abusive verify status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "authentication_rate_limited") {
		t.Fatalf("expected authentication_rate_limited envelope, got %s", second.Body.String())
	}
}

func TestEmailAuthDisabledReturnsNotFoundAndLeavesNimiqWorking(t *testing.T) {
	handler, _ := testNimiqHandler(t, ratelimit.NewMemoryLimiter(), config.NimiqAuthConfig{})
	handler.SetEmailAuthEnabled(false)
	for _, fn := range []func(http.ResponseWriter, *http.Request){handler.Register, handler.Login, handler.IssueToken} {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"password1","first_name":"A","last_name":"B"}`)))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("disabled email auth status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	body := `{"wallet_address":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","network":"test-albatross","environment":"testnet","purpose":"AUTH_LOGIN","transport":"mini-app"}`
	challenge := httptest.NewRecorder()
	handler.CreateNimiqChallenge(challenge, httptest.NewRequest(http.MethodPost, "/api/v1/auth/nimiq/challenges", strings.NewReader(body)))
	if challenge.Code != http.StatusCreated {
		t.Fatalf("nimiq challenge status=%d body=%s", challenge.Code, challenge.Body.String())
	}
}

func TestEmailAuthRegisterAndLoginAreRateLimitedByIPAndNormalizedEmail(t *testing.T) {
	cookie := config.NimiqAuthConfig{
		Network: "test-albatross", Environment: "testnet", Domain: "test.nimnear.local",
		ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax",
		ChallengeIPLimit: 2, VerifyIPLimit: 2, AbuseLimit: 1, RateLimitWindow: time.Minute,
	}
	handler, _ := testNimiqHandler(t, ratelimit.NewMemoryLimiter(), cookie)
	first := httptest.NewRecorder()
	handler.Register(first, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"  FOO@Bar.com "}`)))
	if first.Code != http.StatusBadRequest {
		t.Fatalf("first register status=%d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	handler.Register(second, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"foo@bar.com","password":"password1","first_name":"A","last_name":"B"}`)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("normalized email register status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "authentication_rate_limited") {
		t.Fatalf("expected authentication_rate_limited envelope, got %s", second.Body.String())
	}

	loginFirst := httptest.NewRecorder()
	handler.Login(loginFirst, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"other@example.com"}`)))
	if loginFirst.Code != http.StatusBadRequest {
		t.Fatalf("first login status=%d body=%s", loginFirst.Code, loginFirst.Body.String())
	}
	loginSecond := httptest.NewRecorder()
	handler.IssueToken(loginSecond, httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(`{"email":"OTHER@example.com","password":"password1"}`)))
	if loginSecond.Code != http.StatusTooManyRequests {
		t.Fatalf("token identity status=%d body=%s", loginSecond.Code, loginSecond.Body.String())
	}
}

func TestEmailIdentityKeyDoesNotContainRawEmail(t *testing.T) {
	key := emailIdentityKey("  User@Example.COM ")
	if key == "" || strings.Contains(strings.ToLower(key), "user@example.com") {
		t.Fatalf("identity key leaked email: %q", key)
	}
	if key != emailIdentityKey("user@example.com") {
		t.Fatal("identity key must be stable across case and surrounding space")
	}
}
