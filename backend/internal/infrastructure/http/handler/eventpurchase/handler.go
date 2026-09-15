package eventpurchase

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler exposes authenticated purchase and payment endpoints.
type Handler struct {
	uc *usecase.PurchaseUseCase
}

// NewHandler creates a purchase handler.
func NewHandler(uc *usecase.PurchaseUseCase) *Handler {
	return &Handler{uc: uc}
}

// Create creates an idempotent pending purchase. It accepts no client amount or payment proof.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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
