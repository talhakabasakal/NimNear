package calendar

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/calendar/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/calendar/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

// Handler serves public calendar reads and JWT-protected calendar mutations.
type Handler struct {
	calendarUC *usecase.CalendarUseCase
}

// NewHandler creates a calendar handler.
func NewHandler(calendarUC *usecase.CalendarUseCase) *Handler {
	return &Handler{calendarUC: calendarUC}
}

func (h *Handler) ListPublic(w http.ResponseWriter, r *http.Request) {
	limit := usecase.DefaultCalendarLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(w, domainErr.New(domainErr.ErrBadRequest, "limit must be a valid integer", nil))
			return
		}
		limit = parsed
	}
	if h.calendarUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "calendar service is not configured", nil))
		return
	}
	result, err := h.calendarUC.ListPublic(r.Context(), limit)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid calendar id", nil))
		return
	}
	if h.calendarUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "calendar service is not configured", nil))
		return
	}
	result, err := h.calendarUC.GetPublic(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil))
		return
	}
	if h.calendarUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "calendar service is not configured", nil))
		return
	}
	result, err := h.calendarUC.ListMine(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil))
		return
	}
	var req dto.CreateCalendarRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid calendar request", nil))
		return
	}
	if h.calendarUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "calendar service is not configured", nil))
		return
	}
	result, err := h.calendarUC.Create(r.Context(), userID, req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, result)
}

func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil))
		return
	}
	calendarID, err := parseID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if err := h.calendarUC.Follow(r.Context(), userID, calendarID); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}

func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil))
		return
	}
	calendarID, err := parseID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if err := h.calendarUC.Unfollow(r.Context(), userID, calendarID); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}

func parseID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, domainErr.New(domainErr.ErrBadRequest, "invalid calendar id", nil)
	}
	return id, nil
}
