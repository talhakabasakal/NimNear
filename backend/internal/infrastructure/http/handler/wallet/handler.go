package wallet

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/wallet/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/wallet/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler exposes authenticated wallet data for the caller's verified Nimiq identities.
type Handler struct {
	uc *usecase.WalletUseCase
}

func NewHandler(uc *usecase.WalletUseCase) *Handler {
	return &Handler{uc: uc}
}

// GetBalance returns the Luna balance for one of the authenticated user's verified identities.
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	query, ok := h.walletQuery(w, r)
	if !ok {
		return
	}
	result, err := h.uc.GetBalance(r.Context(), query)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// GetTransactions returns historic transactions for one of the authenticated user's verified identities.
func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	query, ok := h.walletQuery(w, r)
	if !ok {
		return
	}
	if max := strings.TrimSpace(r.URL.Query().Get("max")); max != "" {
		parsed, err := strconv.Atoi(max)
		if err != nil {
			response.Error(w, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_transaction_limit", "max must be a positive integer", nil))
			return
		}
		query.Max = parsed
	}
	query.StartAt = strings.TrimSpace(r.URL.Query().Get("start_at"))
	result, err := h.uc.GetTransactions(r.Context(), query)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) walletQuery(w http.ResponseWriter, r *http.Request) (dto.WalletQuery, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return dto.WalletQuery{}, false
	}
	if h.uc == nil {
		response.Error(w, domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq wallet data is not configured", nil))
		return dto.WalletQuery{}, false
	}
	return dto.WalletQuery{
		UserID:  userID,
		Address: strings.TrimSpace(r.URL.Query().Get("address")),
	}, true
}
