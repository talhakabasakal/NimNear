package paymentrequest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/httpx"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

const (
	defaultCreateLimit = 20
	defaultLookupLimit = 60
	defaultSubmitLimit = 30
)

// Limits bound create, public lookup, and transaction-submission abuse.
type Limits struct {
	Create int
	Lookup int
	Submit int
	Window time.Duration
}

// Handler exposes payment-request endpoints.
type Handler struct {
	uc      *usecase.RequestUseCase
	limiter ratelimit.Limiter
	limits  Limits
}

func NewHandler(uc *usecase.RequestUseCase, limiter ratelimit.Limiter, limits Limits) *Handler {
	if limits.Create <= 0 {
		limits.Create = defaultCreateLimit
	}
	if limits.Lookup <= 0 {
		limits.Lookup = defaultLookupLimit
	}
	if limits.Submit <= 0 {
		limits.Submit = defaultSubmitLimit
	}
	if limits.Window <= 0 {
		limits.Window = time.Minute
	}
	return &Handler{uc: uc, limiter: limiter, limits: limits}
}

// Create creates a pending payment request for the authenticated user's verified identity.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	if err := h.limit(r, "payment_request:create:user:"+userID.String(), h.limits.Create); err != nil {
		response.Error(w, err)
		return
	}
	input, err := decodeCreate(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.Create(r.Context(), userID, input)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, result)
}

// List returns the authenticated creator's payment requests.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(w, domainErr.New(domainErr.ErrBadRequest, "limit must be between 1 and 100", nil))
			return
		}
		limit = parsed
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.ListOwned(r.Context(), userID, limit)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Get returns a creator-scoped payment request.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	publicID, err := parsePublicID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.GetOwned(r.Context(), publicID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// GetPublic returns the payer-needed subset of a payment request without authentication.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	if err := h.limit(r, "payment_request:public:ip:"+httpx.FromRequest(r), h.limits.Lookup); err != nil {
		response.Error(w, err)
		return
	}
	publicID, err := parsePublicID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.GetPublic(r.Context(), publicID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Cancel lets the creator cancel a pending payment request.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	publicID, err := parsePublicID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.Cancel(r.Context(), publicID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// SubmitTransaction accepts only the hash returned by Nimiq Pay from an authenticated payer.
func (h *Handler) SubmitTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	if err := h.limit(r, "payment_request:submit:user:"+userID.String(), h.limits.Submit); err != nil {
		response.Error(w, err)
		return
	}
	publicID, err := parsePublicID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	var input dto.SubmitTransactionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "request must contain only transaction_hash", err))
		return
	}
	result, err := h.uc.SubmitTransaction(r.Context(), publicID, userID, input.TransactionHash)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, result)
}

// RecoverExpired re-verifies an expired request that already has a reserved hash.
func (h *Handler) RecoverExpired(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	publicID, err := parsePublicID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "payment request service is not configured", nil))
		return
	}
	result, err := h.uc.RecoverExpired(r.Context(), publicID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func decodeCreate(r *http.Request) (dto.CreateRequest, error) {
	var input dto.CreateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return dto.CreateRequest{}, domainErr.New(domainErr.ErrBadRequest, "request must contain only amount_nim, note, and address", err)
	}
	return input, nil
}

func parsePublicID(r *http.Request) (uuid.UUID, error) {
	publicID, err := uuid.Parse(chi.URLParam(r, "public_id"))
	if err != nil {
		return uuid.Nil, domainErr.New(domainErr.ErrBadRequest, "invalid payment request id", nil)
	}
	return publicID, nil
}

func (h *Handler) limit(r *http.Request, key string, limit int) error {
	if h == nil || h.limiter == nil || limit <= 0 {
		return nil
	}
	err := h.limiter.Allow(r.Context(), key, limit, h.limits.Window)
	if err == nil {
		return nil
	}
	if domainErr.ErrorCode(err) == "authentication_rate_limited" {
		return domainErr.NewWithCode(domainErr.ErrRateLimited, "rate_limited", "too many requests", nil)
	}
	return err
}
