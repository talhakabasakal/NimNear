package usecase

import (
	"context"
	"math"

	"github.com/masterfabric-go/masterfabric/internal/application/place/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	// DefaultRadiusMeters keeps an omitted radius useful without allowing an unbounded query.
	DefaultRadiusMeters = 5000.0
	// MaxRadiusMeters bounds work performed by the public discovery endpoint.
	MaxRadiusMeters = 50000.0
	// MaxNearbyPlaces bounds the response size for a discovery request.
	MaxNearbyPlaces = 100
)

// NearbyPlacesUseCase handles active place discovery around a coordinate.
type NearbyPlacesUseCase struct {
	placeRepo repository.PlaceRepository
}

// NewNearbyPlacesUseCase creates a nearby places use case.
func NewNearbyPlacesUseCase(placeRepo repository.PlaceRepository) *NearbyPlacesUseCase {
	return &NearbyPlacesUseCase{placeRepo: placeRepo}
}

// Execute returns active places ordered by distance from the requested point.
func (uc *NearbyPlacesUseCase) Execute(ctx context.Context, latitude, longitude, radiusMeters float64) (*dto.NearbyPlacesResponse, error) {
	if err := validateCoordinates(latitude, longitude); err != nil {
		return nil, err
	}
	if math.IsNaN(radiusMeters) || math.IsInf(radiusMeters, 0) || radiusMeters <= 0 || radiusMeters > MaxRadiusMeters {
		return nil, domainErr.New(domainErr.ErrBadRequest, "radius must be greater than 0 and no more than 50000 meters", nil)
	}
	if uc.placeRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "place repository is not configured", nil)
	}

	places, err := uc.placeRepo.ListNearby(ctx, latitude, longitude, radiusMeters, MaxNearbyPlaces)
	if err != nil {
		return nil, err
	}

	data := make([]dto.NearbyPlaceInfo, 0, len(places))
	for _, place := range places {
		if place == nil {
			continue
		}
		data = append(data, dto.NearbyPlaceInfo{
			ID:             place.ID,
			Name:           place.Name,
			Description:    place.Description,
			Latitude:       place.Latitude,
			Longitude:      place.Longitude,
			Address:        place.Address,
			Category:       place.Category,
			ImageURL:       place.ImageURL,
			DistanceMeters: int64(math.Round(place.DistanceMeters)),
		})
	}

	return &dto.NearbyPlacesResponse{Data: data}, nil
}

func validateCoordinates(latitude, longitude float64) error {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return domainErr.New(domainErr.ErrBadRequest, "latitude must be between -90 and 90", nil)
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return domainErr.New(domainErr.ErrBadRequest, "longitude must be between -180 and 180", nil)
	}
	return nil
}
