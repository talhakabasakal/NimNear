package place

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler provides public place discovery HTTP handlers.
type Handler struct {
	nearbyPlacesUC *usecase.NearbyPlacesUseCase
}

// NewHandler creates a place handler.
func NewHandler(nearbyPlacesUC *usecase.NearbyPlacesUseCase) *Handler {
	return &Handler{nearbyPlacesUC: nearbyPlacesUC}
}

// ListNearby returns active places near the requested coordinate.
func (h *Handler) ListNearby(w http.ResponseWriter, r *http.Request) {
	latitude, err := parseRequiredFloat(r, "lat")
	if err != nil {
		response.Error(w, err)
		return
	}
	longitude, err := parseRequiredFloat(r, "lng")
	if err != nil {
		response.Error(w, err)
		return
	}

	radius := usecase.DefaultRadiusMeters
	if rawRadius, ok := r.URL.Query()["radius"]; ok {
		if len(rawRadius) == 0 || strings.TrimSpace(rawRadius[0]) == "" {
			response.Error(w, domainErr.New(domainErr.ErrBadRequest, "radius must be a valid number", nil))
			return
		}
		radius, err = parseFloat(rawRadius[0], "radius")
		if err != nil {
			response.Error(w, err)
			return
		}
	}

	if h.nearbyPlacesUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "nearby places service is not configured", nil))
		return
	}
	result, err := h.nearbyPlacesUC.Execute(r.Context(), latitude, longitude, radius)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func parseRequiredFloat(r *http.Request, name string) (float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return 0, domainErr.New(domainErr.ErrBadRequest, name+" is required", nil)
	}
	return parseFloat(raw, name)
}

func parseFloat(raw, name string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, domainErr.New(domainErr.ErrBadRequest, name+" must be a valid number", err)
	}
	return value, nil
}

// Get returns one active public place by UUID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, domainErr.New(domainErr.ErrBadRequest, "invalid place id", nil))
		return
	}
	if h.nearbyPlacesUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "place service is not configured", nil))
		return
	}
	result, err := h.nearbyPlacesUC.Get(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, result)
}
