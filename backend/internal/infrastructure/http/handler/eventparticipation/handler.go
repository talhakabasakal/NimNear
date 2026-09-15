package eventparticipation

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventparticipation/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler exposes authenticated RSVP operations.
type Handler struct {
	uc *usecase.ParticipationUseCase
}

// NewHandler creates a participation handler.
func NewHandler(uc *usecase.ParticipationUseCase) *Handler {
	return &Handler{uc: uc}
}

// RSVP adds the authenticated user to an eligible free event.
func (h *Handler) RSVP(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(eventID, userID uuid.UUID) (interface{}, error) {
		if h.uc == nil {
			return nil, domainErr.New(domainErr.ErrInternal, "participation service is not configured", nil)
		}
		return h.uc.RSVP(r.Context(), eventID, userID)
	})
}

// Cancel removes the authenticated user's RSVP.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(eventID, userID uuid.UUID) (interface{}, error) {
		if h.uc == nil {
			return nil, domainErr.New(domainErr.ErrInternal, "participation service is not configured", nil)
		}
		return h.uc.Cancel(r.Context(), eventID, userID)
	})
}

// GetState returns the authenticated user's RSVP state.
func (h *Handler) GetState(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(eventID, userID uuid.UUID) (interface{}, error) {
		if h.uc == nil {
			return nil, domainErr.New(domainErr.ErrInternal, "participation service is not configured", nil)
		}
		return h.uc.GetState(r.Context(), eventID, userID)
	})
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, operation func(uuid.UUID, uuid.UUID) (interface{}, error)) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid event id", nil))
		return
	}
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	result, err := operation(eventID, userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}
