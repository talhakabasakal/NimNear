package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakePlaceRepository struct {
	places       []*model.NearbyPlace
	err          error
	latitude     float64
	longitude    float64
	radiusMeters float64
	limit        int
}

func (f *fakePlaceRepository) ListNearby(_ context.Context, latitude, longitude, radiusMeters float64, limit int) ([]*model.NearbyPlace, error) {
	f.latitude = latitude
	f.longitude = longitude
	f.radiusMeters = radiusMeters
	f.limit = limit
	return f.places, f.err
}

func (f *fakePlaceRepository) GetActiveByID(_ context.Context, _ uuid.UUID) (*model.Place, error) {
	return nil, f.err
}

func TestNearbyPlacesUseCaseExecuteRejectsZeroRadius(t *testing.T) {
	repo := &fakePlaceRepository{}
	uc := NewNearbyPlacesUseCase(repo)

	_, err := uc.Execute(context.Background(), 41.0082, 28.9784, 0)
	if err == nil || !errors.Is(err, domainErr.ErrBadRequest) {
		t.Fatalf("error = %v, want bad request", err)
	}
}

func TestNearbyPlacesUseCaseExecuteRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name                 string
		latitude             float64
		longitude            float64
		radius               float64
		wantMessageSubstring string
	}{
		{name: "latitude too high", latitude: 91, longitude: 0, radius: 1000, wantMessageSubstring: "latitude"},
		{name: "longitude too low", latitude: 0, longitude: -181, radius: 1000, wantMessageSubstring: "longitude"},
		{name: "radius too large", latitude: 0, longitude: 0, radius: MaxRadiusMeters + 1, wantMessageSubstring: "radius"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewNearbyPlacesUseCase(&fakePlaceRepository{}).Execute(context.Background(), tt.latitude, tt.longitude, tt.radius)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, domainErr.ErrBadRequest) {
				t.Fatalf("error = %v, want bad request", err)
			}
			if !strings.Contains(err.Error(), tt.wantMessageSubstring) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantMessageSubstring)
			}
		})
	}
}

func TestNearbyPlacesUseCaseExecuteMapsPlaces(t *testing.T) {
	id := uuid.New()
	repo := &fakePlaceRepository{
		places: []*model.NearbyPlace{{
			Place: model.Place{
				ID:          id,
				Name:        "NIMNear Cafe",
				Description: "A nearby place",
				Latitude:    41.01,
				Longitude:   28.98,
				Address:     "Istanbul",
				Category:    "cafe",
				ImageURL:    "https://example.com/place.jpg",
			},
			DistanceMeters: 239.6,
		}},
	}
	result, err := NewNearbyPlacesUseCase(repo).Execute(context.Background(), 41.0082, 28.9784, 1000)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("got %d places, want 1", len(result.Data))
	}
	got := result.Data[0]
	if got.ID != id || got.Name != "NIMNear Cafe" || got.DistanceMeters != 240 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestNearbyPlacesUseCaseExecuteReturnsEmptyResponse(t *testing.T) {
	result, err := NewNearbyPlacesUseCase(&fakePlaceRepository{}).Execute(context.Background(), 41.0082, 28.9784, 1000)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Data == nil || len(result.Data) != 0 {
		t.Fatalf("unexpected empty result: %#v", result)
	}
}

func TestNearbyPlacesUseCaseGetMapsActivePlace(t *testing.T) {
	id := uuid.New()
	result, err := NewNearbyPlacesUseCase(&placeDetailRepository{place: &model.Place{ID: id, Name: "NIMNear Cafe", IsActive: true}}).Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if result.Data.ID != id || result.Data.Name != "NIMNear Cafe" {
		t.Fatalf("unexpected detail: %#v", result.Data)
	}
}

type placeDetailRepository struct {
	place *model.Place
}

func (r *placeDetailRepository) ListNearby(context.Context, float64, float64, float64, int) ([]*model.NearbyPlace, error) {
	return nil, nil
}

func (r *placeDetailRepository) GetActiveByID(context.Context, uuid.UUID) (*model.Place, error) {
	return r.place, nil
}
