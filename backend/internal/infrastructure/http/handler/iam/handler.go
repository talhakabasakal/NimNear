package iam

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/httpx"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"

	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// Handler provides IAM HTTP handlers.
type Handler struct {
	registerUC       *usecase.RegisterUseCase
	loginUC          *usecase.LoginUseCase
	assignRoleUC     *usecase.AssignRoleUseCase
	userRepo         iamRepo.UserRepository
	nimiqAuth        *usecase.NimiqAuthUseCase
	cookie           config.NimiqAuthConfig
	sessionTTL       time.Duration
	limiter          ratelimit.Limiter
	emailAuthEnabled bool
	deleteAccountUC  *usecase.DeleteAccountUseCase
}

// NewHandler creates a new IAM handler.
func NewHandler(
	registerUC *usecase.RegisterUseCase,
	loginUC *usecase.LoginUseCase,
	assignRoleUC *usecase.AssignRoleUseCase,
	userRepo iamRepo.UserRepository,
	nimiqAuth *usecase.NimiqAuthUseCase,
	cookie config.NimiqAuthConfig,
	sessionTTL time.Duration,
	limiter ratelimit.Limiter,
) *Handler {
	return &Handler{
		registerUC:       registerUC,
		loginUC:          loginUC,
		assignRoleUC:     assignRoleUC,
		userRepo:         userRepo,
		nimiqAuth:        nimiqAuth,
		cookie:           cookie,
		sessionTTL:       sessionTTL,
		limiter:          limiter,
		emailAuthEnabled: true,
	}
}

// SetEmailAuthEnabled controls the legacy email/password surface.
func (h *Handler) SetEmailAuthEnabled(enabled bool) {
	if h == nil {
		return
	}
	h.emailAuthEnabled = enabled
}

// SetDeleteAccountUseCase configures self-service account deletion.
func (h *Handler) SetDeleteAccountUseCase(uc *usecase.DeleteAccountUseCase) {
	if h == nil {
		return
	}
	h.deleteAccountUC = uc
}

// Register handles user registration.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.emailAuthEnabled {
		response.Error(w, emailAuthDisabled())
		return
	}
	if err := h.limitEmailAuth(r, "register"); err != nil {
		response.Error(w, err)
		return
	}
	var req dto.RegisterRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, err := h.registerUC.Execute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, user)
}

// Login handles user authentication.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.emailAuthEnabled {
		response.Error(w, emailAuthDisabled())
		return
	}
	if err := h.limitEmailAuth(r, "login"); err != nil {
		response.Error(w, err)
		return
	}
	var req dto.LoginRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.loginUC.Execute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	h.writeSessionCookie(w, result.Token)
	response.JSON(w, http.StatusOK, dto.SessionResponse{User: result.User})
}

// IssueToken authenticates email/password API clients and returns a Bearer JWT.
// Browser cookie sessions must use Login instead; this endpoint does not set a cookie.
func (h *Handler) IssueToken(w http.ResponseWriter, r *http.Request) {
	if !h.emailAuthEnabled {
		response.Error(w, emailAuthDisabled())
		return
	}
	if err := h.limitEmailAuth(r, "login"); err != nil {
		response.Error(w, err)
		return
	}
	var req dto.LoginRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.loginUC.Execute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.TokenResponse{Token: result.Token, User: result.User})
}

// AssignRole handles role assignment.
func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	var req dto.AssignRoleRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.assignRoleUC.Execute(r.Context(), req); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

// GetMe returns the current authenticated user.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	if !user.IsActive() {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "account is not active", nil))
		return
	}

	walletAddress := ""
	if h.nimiqAuth != nil {
		walletAddress, err = h.nimiqAuth.AddressForUser(r.Context(), user.ID)
		if err != nil {
			response.Error(w, err)
			return
		}
	}

	info := dto.ToUserInfo(user, walletAddress)
	response.JSON(w, http.StatusOK, info)
}

// DeleteMe anonymizes the authenticated user and clears the browser session cookie.
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if h.deleteAccountUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "account deletion is not configured", nil))
		return
	}
	if err := h.deleteAccountUC.Execute(r.Context(), userID); err != nil {
		response.Error(w, err)
		return
	}
	cookie := h.sessionCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
	response.NoContent(w)
}

// GetUser returns a user by ID.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Status:    string(user.Status),
		CreatedAt: user.CreatedAt,
	})
}

// ListUsers returns a paginated list of users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	params := pagination.FromRequest(r)

	users, total, err := h.userRepo.List(r.Context(), params.Offset(), params.Limit())
	if err != nil {
		response.Error(w, err)
		return
	}

	var infos []dto.UserInfo
	for _, u := range users {
		infos = append(infos, dto.UserInfo{
			ID:        u.ID,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Status:    string(u.Status),
			CreatedAt: u.CreatedAt,
		})
	}

	response.JSON(w, http.StatusOK, pagination.NewResult(infos, params, total))
}

// CreateNimiqChallenge issues a short-lived, wallet-bound AUTH_LOGIN message.
func (h *Handler) CreateNimiqChallenge(w http.ResponseWriter, r *http.Request) {
	if h.nimiqAuth == nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Nimiq authentication unavailable"})
		return
	}
	if err := h.limitAuth(r, "nimiq_auth:challenge:ip:"+httpx.FromRequest(r), h.cookie.ChallengeLimit()); err != nil {
		response.Error(w, err)
		return
	}
	var req usecase.NimiqChallengeRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if address, err := usecase.NormalizeNimiqAddress(req.Address); err == nil {
		if err := h.limitAuth(r, "nimiq_auth:challenge:addr:"+strings.ReplaceAll(address, " ", ""), h.cookie.ChallengeLimit()); err != nil {
			response.Error(w, err)
			return
		}
	}
	result, err := h.nimiqAuth.CreateChallenge(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	slog.Info("[auth] challenge created", "transport", result.Transport, "network", result.Network)
	response.JSON(w, http.StatusCreated, result)
}

// VerifyNimiqChallenge verifies wallet ownership, consumes the challenge, and creates a session.
func (h *Handler) VerifyNimiqChallenge(w http.ResponseWriter, r *http.Request) {
	if h.nimiqAuth == nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Nimiq authentication unavailable"})
		return
	}
	if err := h.limitAuth(r, "nimiq_auth:verify:ip:"+httpx.FromRequest(r), h.cookie.VerifyLimit()); err != nil {
		response.Error(w, err)
		return
	}
	var req usecase.NimiqVerifyRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.limitAuth(r, "nimiq_auth:verify:challenge:"+req.ChallengeID.String(), h.cookie.VerifyAbuseLimit()); err != nil {
		response.Error(w, err)
		return
	}
	if address := h.nimiqAuth.ChallengeClaimedAddress(r.Context(), req.ChallengeID); address != "" {
		if err := h.limitAuth(r, "nimiq_auth:verify:addr:"+strings.ReplaceAll(address, " ", ""), h.cookie.VerifyAbuseLimit()); err != nil {
			response.Error(w, err)
			return
		}
	}
	result, err := h.nimiqAuth.Verify(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	h.writeSessionCookie(w, result.Token)
	slog.Info("[auth] signature verified and session created", "user_id", result.User.ID)
	response.JSON(w, http.StatusOK, dto.SessionResponse{User: result.User})
}

// Logout clears the host-only authentication cookie.
func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	cookie := h.sessionCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
	response.NoContent(w)
}

func (h *Handler) writeSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, h.sessionCookie(token))
}
func (h *Handler) sessionCookie(value string) *http.Cookie {
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(strings.TrimSpace(h.cookie.CookieSameSite)) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	}
	return &http.Cookie{Name: h.cookie.CookieName, Value: value, Path: "/", HttpOnly: true, Secure: h.cookie.CookieSecure, SameSite: sameSite, MaxAge: int(h.sessionTTL.Seconds())}
}

func (h *Handler) limitAuth(r *http.Request, key string, limit int) error {
	if h == nil || h.limiter == nil || limit <= 0 {
		return nil
	}
	return h.limiter.Allow(r.Context(), key, limit, h.cookie.AuthRateLimitWindow())
}

func (h *Handler) limitEmailAuth(r *http.Request, action string) error {
	if err := h.limitAuth(r, "email_auth:"+action+":ip:"+httpx.FromRequest(r), h.emailAuthIPLimit(action)); err != nil {
		return err
	}
	if identity := emailIdentityKey(peekJSONEmail(r)); identity != "" {
		if err := h.limitAuth(r, "email_auth:"+action+":id:"+identity, h.cookie.VerifyAbuseLimit()); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) emailAuthIPLimit(action string) int {
	if action == "register" {
		return h.cookie.ChallengeLimit()
	}
	return h.cookie.VerifyLimit()
}

func emailAuthDisabled() error {
	return domainErr.New(domainErr.ErrNotFound, "not found", nil)
}

func peekJSONEmail(r *http.Request) string {
	if r == nil || r.Body == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || len(body) == 0 {
		return ""
	}
	var payload struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.Email
}

func emailIdentityKey(email string) string {
	normalized := dto.NormalizeEmail(email)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
