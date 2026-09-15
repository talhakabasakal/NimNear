package profile

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	profiledto "github.com/masterfabric-go/masterfabric/internal/application/profile/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/profile/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler serves privacy-safe public profile endpoints and self-profile updates.
type Handler struct {
	profileUC *usecase.ProfileUseCase
}

// NewHandler creates a profile handler.
func NewHandler(profileUC *usecase.ProfileUseCase) *Handler {
	return &Handler{profileUC: profileUC}
}

// GetPublic returns a public profile by UUID.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid profile id", nil))
		return
	}
	if h.profileUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "profile service is not configured", nil))
		return
	}
	result, err := h.profileUC.GetPublic(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// ListEvents returns organized or attended public events for a profile.
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid profile id", nil))
		return
	}
	if h.profileUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "profile service is not configured", nil))
		return
	}
	result, err := h.profileUC.ListEvents(r.Context(), id, strings.TrimSpace(r.URL.Query().Get("type")))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// UpdateMe updates the authenticated user's profile. The user ID is always
// taken from JWT middleware context and cannot be supplied by the client.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil))
		return
	}
	if h.profileUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "profile service is not configured", nil))
		return
	}

	var req profiledto.UpdateProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid profile update request", nil))
		return
	}

	result, err := h.profileUC.UpdateSelf(r.Context(), userID, req.Patch())
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}
