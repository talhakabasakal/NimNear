package event

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// EventRepo implements public event persistence with PostgreSQL.
type EventRepo struct {
	db *pgxpool.Pool
}

// NewEventRepo creates a PostgreSQL event repository.
func NewEventRepo(db *pgxpool.Pool) *EventRepo {
	return &EventRepo{db: db}
}

// ListPublic applies discovery filters and chronological sorting in PostgreSQL.
func (r *EventRepo) ListPublic(ctx context.Context, filter repository.ListFilter) ([]*model.Event, error) {
	args := make([]any, 0, 4)
	where := "is_public = TRUE AND status = 'published'"
	if filter.City != "" {
		args = append(args, filter.City)
		where += fmt.Sprintf(" AND LOWER(city) = LOWER($%d)", len(args))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		where += fmt.Sprintf(" AND starts_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		where += fmt.Sprintf(" AND starts_at <= $%d", len(args))
	}

	limitPlaceholder := len(args) + 1
	args = append(args, filter.Limit)
	query := fmt.Sprintf(`
		SELECT id, title, description, starts_at, ends_at, status, price_lunas,
		       currency, capacity, attendee_count, image_url, place_id,
		       latitude, longitude, address, city, organizer_id, is_public,
		       created_at, updated_at
		FROM events
		WHERE %s
		ORDER BY starts_at ASC, id ASC
		LIMIT $%d`, where, limitPlaceholder)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list events", err)
	}
	defer rows.Close()

	events := make([]*model.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan event", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read events", err)
	}
	return events, nil
}

// ListPublicByOrganizer returns all public published events organized by a user.
func (r *EventRepo) ListPublicByOrganizer(ctx context.Context, organizerID uuid.UUID) ([]*model.Event, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, title, description, starts_at, ends_at, status, price_lunas, "+
			"currency, capacity, attendee_count, image_url, place_id, latitude, longitude, "+
			"address, city, organizer_id, is_public, created_at, updated_at "+
			"FROM events WHERE organizer_id = $1 AND is_public = TRUE AND status = 'published' "+
			"ORDER BY starts_at ASC, id ASC", organizerID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list organized events", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

// ListPublicByAttendee returns all public published events attended by a user.
func (r *EventRepo) ListPublicByAttendee(ctx context.Context, userID uuid.UUID) ([]*model.Event, error) {
	rows, err := r.db.Query(ctx,
		"SELECT e.id, e.title, e.description, e.starts_at, e.ends_at, e.status, e.price_lunas, "+
			"e.currency, e.capacity, e.attendee_count, e.image_url, e.place_id, e.latitude, e.longitude, "+
			"e.address, e.city, e.organizer_id, e.is_public, e.created_at, e.updated_at "+
			"FROM events e JOIN event_participants ep ON ep.event_id = e.id "+
			"WHERE ep.user_id = $1 AND e.is_public = TRUE AND e.status = 'published' "+
			"ORDER BY e.starts_at ASC, e.id ASC", userID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list attended events", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

func collectEvents(rows pgx.Rows) ([]*model.Event, error) {
	events := make([]*model.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan event", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read events", err)
	}
	return events, nil
}

// GetPublicByID returns only published public events.
func (r *EventRepo) GetPublicByID(ctx context.Context, id uuid.UUID) (*model.Event, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, title, description, starts_at, ends_at, status, price_lunas,
		       currency, capacity, attendee_count, image_url, place_id,
		       latitude, longitude, address, city, organizer_id, is_public,
		       created_at, updated_at
		FROM events
		WHERE id = $1 AND is_public = TRUE AND status = 'published'`, id)

	event, err := scanEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get event", err)
	}
	return event, nil
}

// Create persists a newly published event.
func (r *EventRepo) Create(ctx context.Context, event *model.Event) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO events (
			id, title, description, starts_at, ends_at, status, price_lunas,
			currency, capacity, attendee_count, image_url, place_id,
			latitude, longitude, address, city, organizer_id, is_public,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		event.ID,
		event.Title,
		event.Description,
		event.StartsAt,
		event.EndsAt,
		event.Status,
		event.PriceLunas,
		event.Currency,
		event.Capacity,
		event.AttendeeCount,
		event.ImageURL,
		event.PlaceID,
		event.Latitude,
		event.Longitude,
		event.Address,
		event.City,
		event.OrganizerID,
		event.IsPublic,
		event.CreatedAt,
		event.UpdatedAt,
	)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create event", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (*model.Event, error) {
	var (
		event       model.Event
		placeID     pgtype.UUID
		latitude    pgtype.Float8
		longitude   pgtype.Float8
		address     pgtype.Text
		organizerID pgtype.UUID
	)
	if err := row.Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.StartsAt,
		&event.EndsAt,
		&event.Status,
		&event.PriceLunas,
		&event.Currency,
		&event.Capacity,
		&event.AttendeeCount,
		&event.ImageURL,
		&placeID,
		&latitude,
		&longitude,
		&address,
		&event.City,
		&organizerID,
		&event.IsPublic,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if placeID.Valid {
		id := uuid.UUID(placeID.Bytes)
		event.PlaceID = &id
	}
	if latitude.Valid {
		value := latitude.Float64
		event.Latitude = &value
	}
	if longitude.Valid {
		value := longitude.Float64
		event.Longitude = &value
	}
	if address.Valid {
		value := address.String
		event.Address = &value
	}
	if organizerID.Valid {
		id := uuid.UUID(organizerID.Bytes)
		event.OrganizerID = &id
	}
	return &event, nil
}
