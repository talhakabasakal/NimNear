package place

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		ORDER BY distance_meters ASC, id ASC
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

// GetActiveByID returns an active place by its stable public ID.
func (r *PlaceRepo) GetActiveByID(ctx context.Context, id uuid.UUID) (*model.Place, error) {
	var place model.Place
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, latitude, longitude, address, category, image_url,
		       is_active, created_at, updated_at
		FROM places
		WHERE id = $1 AND is_active = TRUE`, id).Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "place not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get place", err)
	}
	return &place, nil
}

const placeSelectColumns = `id, name, description, latitude, longitude, address, category, image_url,
		       is_active, created_at, updated_at`

// Create inserts an operator-managed place. A preset ID is preserved for
// deterministic development seeds.
func (r *PlaceRepo) Create(ctx context.Context, place *model.Place) error {
	if place.ID == uuid.Nil {
		place.ID = uuid.New()
	}
	now := time.Now().UTC()
	if place.CreatedAt.IsZero() {
		place.CreatedAt = now
	}
	place.UpdatedAt = now
	_, err := r.db.Exec(ctx, `
		INSERT INTO places (id, name, description, latitude, longitude, address, category, image_url, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		place.ID, place.Name, place.Description, place.Latitude, place.Longitude, place.Address, place.Category, place.ImageURL, place.IsActive, place.CreatedAt, place.UpdatedAt,
	)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create place", err)
	}
	return nil
}

// Update persists an operator patch, including inactive records.
func (r *PlaceRepo) Update(ctx context.Context, place *model.Place) error {
	place.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
		UPDATE places
		SET name = $1, description = $2, latitude = $3, longitude = $4, address = $5,
		    category = $6, image_url = $7, is_active = $8, updated_at = $9
		WHERE id = $10`,
		place.Name, place.Description, place.Latitude, place.Longitude, place.Address, place.Category, place.ImageURL, place.IsActive, place.UpdatedAt, place.ID,
	)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to update place", err)
	}
	if tag.RowsAffected() == 0 {
		return domainErr.New(domainErr.ErrNotFound, "place not found", nil)
	}
	return nil
}

// GetByID returns a place regardless of active state.
func (r *PlaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Place, error) {
	var place model.Place
	err := r.db.QueryRow(ctx, `
		SELECT `+placeSelectColumns+`
		FROM places
		WHERE id = $1`, id).Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "place not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get place", err)
	}
	return &place, nil
}

// List returns operator inventory including inactive places.
func (r *PlaceRepo) List(ctx context.Context, limit int) ([]*model.Place, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+placeSelectColumns+`
		FROM places
		ORDER BY name ASC, id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list places", err)
	}
	defer rows.Close()
	return scanPlaces(rows)
}

// ListByName returns places whose trimmed name matches case-insensitively.
func (r *PlaceRepo) ListByName(ctx context.Context, name string, limit int) ([]*model.Place, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+placeSelectColumns+`
		FROM places
		WHERE LOWER(name) = LOWER($1)
		ORDER BY created_at ASC, id ASC
		LIMIT $2`, strings.TrimSpace(name), limit)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list places by name", err)
	}
	defer rows.Close()
	return scanPlaces(rows)
}

func scanPlaces(rows pgx.Rows) ([]*model.Place, error) {
	places := make([]*model.Place, 0)
	for rows.Next() {
		var place model.Place
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
		); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan place", err)
		}
		places = append(places, &place)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read places", err)
	}
	return places, nil
}
