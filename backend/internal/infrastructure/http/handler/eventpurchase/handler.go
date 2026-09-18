package eventpurchase

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

const (
	defaultCreateLimit = 20
	defaultSubmitLimit = 30
)

// Limits bound create and transaction-submission abuse using authenticated user identity.
type Limits struct {
	Create int
	Submit int
	Window time.Duration
}

// Handler exposes authenticated purchase and payment endpoints.
type Handler struct {
	uc      *usecase.PurchaseUseCase
	limiter ratelimit.Limiter
	limits  Limits
}

// NewHandler creates a purchase handler.
func NewHandler(uc *usecase.PurchaseUseCase, limiter ratelimit.Limiter, limits Limits) *Handler {
	if limits.Create <= 0 {
		limits.Create = defaultCreateLimit
	}
	if limits.Submit <= 0 {
		limits.Submit = defaultSubmitLimit
	}
	if limits.Window <= 0 {
		limits.Window = time.Minute
	}
	return &Handler{uc: uc, limiter: limiter, limits: limits}
}

// Create creates an idempotent pending purchase. It accepts no client amount or payment proof.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	if err := h.limit(r, "event_purchase:create:user:"+userID.String(), h.limits.Create); err != nil {
		response.Error(w, err)
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid event id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}
	result, err := h.uc.Create(r.Context(), eventID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, result)
}

// GetCurrent returns an owner's active purchase for an event and safely retries verification.
func (h *Handler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid event id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}
	result, err := h.uc.GetActiveForEvent(r.Context(), eventID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Get returns a purchase only to its owner and safely retries verification.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid purchase id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}
	result, err := h.uc.GetOwned(r.Context(), purchaseID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// PaymentInstructions returns only server-authoritative public payment values.
func (h *Handler) PaymentInstructions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid purchase id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}
	result, err := h.uc.PaymentInstructions(r.Context(), purchaseID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// SubmitTransaction accepts only the hash returned by Nimiq Pay.
func (h *Handler) SubmitTransaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	if err := h.limit(r, "event_purchase:submit:user:"+userID.String(), h.limits.Submit); err != nil {
		response.Error(w, err)
		return
	}
	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid purchase id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}

	var input dto.SubmitTransactionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "request must contain only transaction_hash", err))
		return
	}
	result, err := h.uc.SubmitTransaction(r.Context(), purchaseID, userID, input.TransactionHash)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, result)
}

// RecoverExpired re-verifies an owner-scoped expired purchase that already has a reserved hash.
func (h *Handler) RecoverExpired(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid purchase id", nil))
		return
	}
	if h.uc == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "purchase service is not configured", nil))
		return
	}
	result, err := h.uc.RecoverExpired(r.Context(), purchaseID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
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
