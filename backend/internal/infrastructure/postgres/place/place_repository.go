package place

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const earthRadiusMeters = 6371000.0

// PlaceRepo implements place discovery persistence with PostgreSQL.
type PlaceRepo struct {
	db *pgxpool.Pool
}

// NewPlaceRepo creates a PostgreSQL place repository.
func NewPlaceRepo(db *pgxpool.Pool) *PlaceRepo {
	return &PlaceRepo{db: db}
}

// ListNearby uses a latitude/longitude bounding box to reduce candidates, then
// applies the Haversine distance calculation for accurate ordering and filtering.
func (r *PlaceRepo) ListNearby(ctx context.Context, latitude, longitude, radiusMeters float64, limit int) ([]*model.NearbyPlace, error) {
	latDelta := radiusMeters / earthRadiusMeters * 180 / math.Pi
	latMin := math.Max(-90, latitude-latDelta)
	latMax := math.Min(90, latitude+latDelta)

	args := []any{latitude, longitude, radiusMeters, latMin, latMax}
	where := "latitude BETWEEN $4 AND $5"

	cosLatitude := math.Cos(latitude * math.Pi / 180)
	if math.Abs(cosLatitude) > 1e-12 {
		longitudeDelta := radiusMeters / (earthRadiusMeters * math.Abs(cosLatitude)) * 180 / math.Pi
		if longitudeDelta < 180 {
			longitudeMin := normalizeLongitude(longitude - longitudeDelta)
			longitudeMax := normalizeLongitude(longitude + longitudeDelta)
			args = append(args, longitudeMin, longitudeMax)
			if longitudeMin <= longitudeMax {
				where += " AND longitude BETWEEN $6 AND $7"
			} else {
				where += " AND (longitude >= $6 OR longitude <= $7)"
			}
		}
	}

	limitPlaceholder := len(args) + 1
	query := fmt.Sprintf(`
		SELECT id, name, description, latitude, longitude, address, category, image_url,
		       is_active, created_at, updated_at, distance_meters
		FROM (
			SELECT id, name, description, latitude, longitude, address, category, image_url,
			       is_active, created_at, updated_at,
			       (6371000.0 * acos(least(1.0, greatest(-1.0,
					cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2))
					+ sin(radians($1)) * sin(radians(latitude))
				)))) AS distance_meters
			FROM places
			WHERE is_active = TRUE AND %s
		) AS candidates
		WHERE distance_meters <= $3
		ORDER BY distance_meters ASC
		LIMIT $%d`, where, limitPlaceholder)

	args = append(args, limit)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list nearby places", err)
	}
	defer rows.Close()

	places := make([]*model.NearbyPlace, 0)
	for rows.Next() {
		var place model.NearbyPlace
		if err := rows.Scan(
			&place.ID,
			&place.Name,
			&place.Description,
			&place.Latitude,
			&place.Longitude,
			&place.Address,
			&place.Category,
			&place.ImageURL,
			&place.IsActive,
			&place.CreatedAt,
			&place.UpdatedAt,
			&place.DistanceMeters,
		); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan nearby place", err)
		}
		places = append(places, &place)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read nearby places", err)
	}

	return places, nil
}

func normalizeLongitude(longitude float64) float64 {
	for longitude < -180 {
		longitude += 360
	}
	for longitude > 180 {
		longitude -= 360
	}
	return longitude
}
