package event

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

// Handler provides event discovery and creation HTTP handlers.
type Handler struct {
	eventUC *usecase.EventUseCase
}

// NewHandler creates an event handler.
func NewHandler(eventUC *usecase.EventUseCase) *Handler {
	return &Handler{eventUC: eventUC}
}

// List returns public events using SQL-backed discovery filters.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	query, err := parseListQuery(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	if h.eventUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "event service is not configured", nil))
		return
	}
	result, err := h.eventUC.List(r.Context(), query)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Get returns one public event by UUID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid event id", nil))
		return
	}
	if h.eventUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "event service is not configured", nil))
		return
	}
	result, err := h.eventUC.Get(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

// Create creates a published public event for the authenticated user.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil))
		return
	}
	var req dto.CreateEventRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid event request", nil))
		return
	}
	if h.eventUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "event service is not configured", nil))
		return
	}
	result, err := h.eventUC.Create(r.Context(), organizerID, req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, result)
}

func parseListQuery(r *http.Request) (dto.ListEventsQuery, error) {
	query := dto.ListEventsQuery{City: strings.TrimSpace(r.URL.Query().Get("city"))}
	if raw := strings.TrimSpace(r.URL.Query().Get("place_id")); raw != "" {
		placeID, err := uuid.Parse(raw)
		if err != nil {
			return dto.ListEventsQuery{}, domainErr.New(domainErr.ErrBadRequest, "place_id must be a valid UUID", nil)
		}
		query.PlaceID = &placeID
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return dto.ListEventsQuery{}, domainErr.New(domainErr.ErrBadRequest, "limit must be a valid integer", nil)
		}
		query.Limit = limit
	}
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		query.From, err = parseTime(raw, "from")
		if err != nil {
			return dto.ListEventsQuery{}, err
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		query.To, err = parseTime(raw, "to")
		if err != nil {
			return dto.ListEventsQuery{}, err
		}
	}
	return query, nil
}

func parseTime(raw, name string) (*time.Time, error) {
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, name+" must be an RFC3339 timestamp", nil)
	}
	value = value.UTC()
	return &value, nil
}
