package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	realtimeUC "github.com/masterfabric-go/masterfabric/internal/application/realtime/usecase"
	iammodel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	infraWS "github.com/masterfabric-go/masterfabric/internal/infrastructure/websocket"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

type handlerAuth struct {
	token  string
	userID uuid.UUID
}

func (handlerAuth) HashPassword(string) (string, error) { return "", nil }
func (handlerAuth) VerifyPassword(string, string) error { return nil }
func (handlerAuth) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "", nil
}
func (a handlerAuth) ValidateToken(_ context.Context, token string) (*service.TokenClaims, error) {
	if token != a.token {
		return nil, errInvalidToken{}
	}
	return &service.TokenClaims{UserID: a.userID}, nil
}

type errInvalidToken struct{}

func (errInvalidToken) Error() string { return "invalid token" }

func TestConnectRejectsMissingAuthentication(t *testing.T) {
	handler := NewHandler(Config{Enabled: true, Hub: infraWS.NewHub(nil, 10)})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	response := httptest.NewRecorder()
	handler.Connect(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestConnectDisabled(t *testing.T) {
	handler := NewHandler(Config{Enabled: false, Hub: infraWS.NewHub(nil, 10)})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	response := httptest.NewRecorder()
	handler.Connect(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestConnectAcceptsSessionCookieWithoutAppHeaders(t *testing.T) {
	userID := uuid.New()
	auth := handlerAuth{token: "session-cookie", userID: userID}
	hub := infraWS.NewHub(nil, 10)
	handler := NewHandler(Config{
		Enabled:     true,
		AuthService: auth,
		Hub:         hub,
		Upgrader:    infraWS.NewUpgrader(infraWS.UpgraderConfig{AllowEmptyOrigin: true}),
		CookieName:  "nimnear_session",
		ValidateUC:  realtimeUC.NewValidateConnectUseCase(nil, nil),
	})
	server := httptest.NewServer(http.HandlerFunc(handler.Connect))
	defer server.Close()

	header := http.Header{}
	header.Set("Cookie", "nimnear_session=session-cookie")
	conn, _, err := gorillaws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg["type"] != "subscribed" {
		t.Fatalf("welcome = %#v", msg)
	}
}

func TestConnectAcceptsContextUserWithoutQueryJWT(t *testing.T) {
	userID := uuid.New()
	hub := infraWS.NewHub(nil, 10)
	handler := NewHandler(Config{
		Enabled:    true,
		Hub:        hub,
		Upgrader:   infraWS.NewUpgrader(infraWS.UpgraderConfig{AllowEmptyOrigin: true}),
		ValidateUC: realtimeUC.NewValidateConnectUseCase(nil, nil),
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "token=") {
			t.Fatal("jwt was passed in the websocket query string")
		}
		ctx := context.WithValue(r.Context(), middleware.ContextKeyUserID, userID)
		handler.Connect(w, r.WithContext(ctx))
	}))
	defer server.Close()

	conn, _, err := gorillaws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read: %v", err)
	}
}

func TestConnectRejectsInvalidToken(t *testing.T) {
	handler := NewHandler(Config{
		Enabled:     true,
		AuthService: handlerAuth{token: "good", userID: uuid.New()},
		Hub:         infraWS.NewHub(nil, 10),
		CookieName:  "nimnear_session",
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "bad"})
	response := httptest.NewRecorder()
	handler.Connect(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestConnectRejectsQueryToken(t *testing.T) {
	handler := NewHandler(Config{
		Enabled:     true,
		AuthService: handlerAuth{token: "query-token", userID: uuid.New()},
		Hub:         infraWS.NewHub(nil, 10),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token=query-token", nil)
	response := httptest.NewRecorder()
	handler.Connect(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

type handlerUserRepo struct {
	user *iammodel.User
}

func (handlerUserRepo) Create(context.Context, *iammodel.User) error { return nil }
func (r handlerUserRepo) GetByID(context.Context, uuid.UUID) (*iammodel.User, error) {
	return r.user, nil
}
func (handlerUserRepo) GetByEmail(context.Context, string) (*iammodel.User, error) { return nil, nil }
func (handlerUserRepo) Update(context.Context, *iammodel.User) error               { return nil }
func (handlerUserRepo) Delete(context.Context, uuid.UUID) error                    { return nil }
func (handlerUserRepo) List(context.Context, int, int) ([]*iammodel.User, int, error) {
	return nil, 0, nil
}

func TestConnectRejectsDeletedUser(t *testing.T) {
	userID := uuid.New()
	deleted := time.Now().UTC()
	auth := handlerAuth{token: "session-cookie", userID: userID}
	handler := NewHandler(Config{
		Enabled:     true,
		AuthService: auth,
		Hub:         infraWS.NewHub(nil, 10),
		CookieName:  "nimnear_session",
		ValidateUC: realtimeUC.NewValidateConnectUseCase(nil, nil).WithUsers(handlerUserRepo{
			user: &iammodel.User{ID: userID, Status: iammodel.UserStatusInactive, DeletedAt: &deleted},
		}),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	request.AddCookie(&http.Cookie{Name: "nimnear_session", Value: "session-cookie"})
	response := httptest.NewRecorder()
	handler.Connect(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
