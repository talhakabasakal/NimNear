package place

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
)

type handlerPlaceRepository struct{}

func (handlerPlaceRepository) ListNearby(_ context.Context, _, _, _ float64, _ int) ([]*model.NearbyPlace, error) {
	return []*model.NearbyPlace{}, nil
}

func (handlerPlaceRepository) GetActiveByID(_ context.Context, id uuid.UUID) (*model.Place, error) {
	return &model.Place{ID: id, Name: "NIMNear Cafe", IsActive: true}, nil
}

func TestListNearbyRejectsMissingCoordinates(t *testing.T) {
	handler := NewHandler(usecase.NewNearbyPlacesUseCase(handlerPlaceRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/places/nearby?lat=41.0082", nil)
	rec := httptest.NewRecorder()

	handler.ListNearby(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListNearbyReturnsDataEnvelope(t *testing.T) {
	handler := NewHandler(usecase.NewNearbyPlacesUseCase(handlerPlaceRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/places/nearby?lat=41.0082&lng=28.9784", nil)
	rec := httptest.NewRecorder()

	handler.ListNearby(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := response["data"]; !ok {
		t.Fatal("expected data field")
	}
}

func TestGetReturnsActivePlaceEnvelope(t *testing.T) {
	handler := NewHandler(usecase.NewNearbyPlacesUseCase(handlerPlaceRepository{}))
	placeID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/places/"+placeID, nil)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", placeID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
