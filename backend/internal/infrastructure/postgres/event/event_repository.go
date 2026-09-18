package event

import (
	"context"
	"errors"
	"fmt"
	"time"

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

const eventProjection = `e.id, e.title, e.description, e.starts_at, e.ends_at, e.status, e.price_lunas,
       e.currency, e.capacity,
       (SELECT COUNT(*)::int FROM (
           SELECT p.user_id FROM event_participants p WHERE p.event_id = e.id
           UNION
           SELECT p.user_id FROM event_purchases p WHERE p.event_id = e.id AND p.status = 'confirmed'
       ) effective_attendees) AS attendee_count,
       e.image_url, e.calendar_id, e.place_id, e.latitude, e.longitude, e.address, e.city, e.organizer_id, e.is_public,
       e.created_at, e.updated_at,
       (e.capacity IS NOT NULL AND (SELECT COUNT(*) FROM (
           SELECT p.user_id FROM event_participants p WHERE p.event_id = e.id
           UNION
           SELECT p.user_id FROM event_purchases p WHERE p.event_id = e.id AND p.status IN ('confirmed', 'submitted', 'verifying')
           UNION
           SELECT p.user_id FROM event_purchases p WHERE p.event_id = e.id AND p.status = 'pending' AND (p.capacity_hold_expires_at IS NULL OR p.capacity_hold_expires_at > NOW())
       ) active_attendees) >= e.capacity) AS sold_out`

// ListPublic applies discovery filters and chronological sorting in PostgreSQL.
func (r *EventRepo) ListPublic(ctx context.Context, filter repository.ListFilter) ([]*model.Event, error) {
	args := make([]any, 0, 4)
	where := "is_public = TRUE AND status = 'published'"
	if filter.City != "" {
		args = append(args, filter.City)
		where += fmt.Sprintf(" AND LOWER(city) = LOWER($%d)", len(args))
	}
	if filter.PlaceID != nil {
		args = append(args, *filter.PlaceID)
		where += fmt.Sprintf(" AND place_id = $%d", len(args))
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
		SELECT %s
		FROM events e
		WHERE %s
		ORDER BY e.starts_at ASC, e.id ASC
		LIMIT $%d`, eventProjection, where, limitPlaceholder)

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
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.organizer_id = $1 AND e.is_public = TRUE AND e.status = 'published' ORDER BY e.starts_at ASC, e.id ASC`, eventProjection), organizerID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list organized events", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

// ListPublicByAttendee returns all public published events attended by a user.
func (r *EventRepo) ListPublicByAttendee(ctx context.Context, userID uuid.UUID) ([]*model.Event, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM events e JOIN (SELECT event_id FROM event_participants WHERE user_id = $1 UNION SELECT event_id FROM event_purchases WHERE user_id = $1 AND status = 'confirmed') attended ON attended.event_id = e.id WHERE e.is_public = TRUE AND e.status = 'published' ORDER BY e.starts_at ASC, e.id ASC`, eventProjection), userID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list attended events", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

// ListPublicByCalendar returns published public events associated with a public calendar.
func (r *EventRepo) ListPublicByCalendar(ctx context.Context, calendarID uuid.UUID) ([]*model.Event, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.calendar_id = $1 AND e.is_public = TRUE AND e.status = 'published' ORDER BY e.starts_at ASC, e.id ASC`, eventProjection), calendarID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list calendar events", err)
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
	row := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1 AND e.is_public = TRUE AND e.status IN ('published', 'cancelled')`, eventProjection), id)

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
			currency, capacity, attendee_count, image_url, calendar_id, place_id,
			latitude, longitude, address, city, organizer_id, is_public,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`,
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
		event.CalendarID,
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

// UpdateOwned applies a validated organizer patch while holding the event row.
func (r *EventRepo) UpdateOwned(ctx context.Context, eventID, organizerID uuid.UUID, patch model.Patch, now time.Time) (*model.Event, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin event update", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current model.Event
	if err := scanEventInto(tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1 AND e.organizer_id = $2 AND e.is_public = TRUE FOR UPDATE`, eventProjection), eventID, organizerID), &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock event for update", err)
	}
	if current.Status != model.EventStatusPublished {
		return nil, domainErr.New(domainErr.ErrConflict, "only published events can be edited", nil)
	}
	if (patch.StartsAtSet || patch.EndsAtSet) && !current.StartsAt.After(now) {
		return nil, domainErr.New(domainErr.ErrConflict, "started events cannot have their schedule edited", nil)
	}
	startsAt, endsAt := current.StartsAt, current.EndsAt
	if patch.StartsAtSet {
		startsAt = patch.StartsAt
	}
	if patch.EndsAtSet {
		endsAt = patch.EndsAt
	}
	if !endsAt.After(startsAt) {
		return nil, domainErr.New(domainErr.ErrValidation, "ends_at must be after starts_at", nil)
	}
	if patch.CapacitySet && patch.Capacity != nil {
		var occupied int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*)::int FROM (
			SELECT user_id FROM event_participants WHERE event_id = $1
			UNION
			SELECT user_id FROM event_purchases WHERE event_id = $1 AND status IN ('confirmed', 'submitted', 'verifying')
			UNION
			SELECT user_id FROM event_purchases WHERE event_id = $1 AND status = 'pending' AND (capacity_hold_expires_at IS NULL OR capacity_hold_expires_at > $2)
		) active_attendees`, eventID, now).Scan(&occupied); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to calculate event occupancy", err)
		}
		if occupied > *patch.Capacity {
			return nil, domainErr.New(domainErr.ErrConflict, "capacity cannot be lower than current occupancy", nil)
		}
	}
	placeChanged := false
	if patch.PlaceIDSet {
		switch {
		case patch.PlaceID == nil && current.PlaceID == nil:
		case patch.PlaceID == nil || current.PlaceID == nil:
			placeChanged = true
		default:
			placeChanged = *patch.PlaceID != *current.PlaceID
		}
	}
	if placeChanged {
		var paidActivity bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM event_purchases WHERE event_id = $1 AND status IN ('pending', 'submitted', 'verifying', 'confirmed'))", eventID).Scan(&paidActivity); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to inspect event payment activity", err)
		}
		if paidActivity {
			return nil, domainErr.New(domainErr.ErrConflict, "location cannot change after paid participation activity", nil)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE events SET
		title = CASE WHEN $3 THEN $4 ELSE title END,
		description = CASE WHEN $5 THEN $6 ELSE description END,
		starts_at = CASE WHEN $7 THEN $8 ELSE starts_at END,
		ends_at = CASE WHEN $9 THEN $10 ELSE ends_at END,
		image_url = CASE WHEN $11 THEN $12 ELSE image_url END,
		capacity = CASE WHEN $13 THEN $14::int ELSE capacity END,
		place_id = CASE WHEN $15 THEN $16::uuid ELSE place_id END,
		updated_at = $17
	WHERE id = $1 AND organizer_id = $2`, eventID, organizerID, patch.TitleSet, patch.Title, patch.DescriptionSet, patch.Description, patch.StartsAtSet, startsAt, patch.EndsAtSet, endsAt, patch.ImageURLSet, patch.ImageURL, patch.CapacitySet, patch.Capacity, patch.PlaceIDSet, patch.PlaceID, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to update event", err)
	}
	var updated model.Event
	if err := scanEventInto(tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1`, eventProjection), eventID), &updated); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read updated event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit event update", err)
	}
	return &updated, nil
}

// CancelOwned changes lifecycle state without deleting event or payment evidence.
func (r *EventRepo) CancelOwned(ctx context.Context, eventID, organizerID uuid.UUID, now time.Time) (*model.Event, error) {
	return r.cancel(ctx, eventID, &organizerID, now)
}

// Cancel is the operator/domain cancellation path. It uses the same status
// transition as organizer cancellation and does not alter payment evidence.
func (r *EventRepo) Cancel(ctx context.Context, eventID uuid.UUID, now time.Time) (*model.Event, error) {
	return r.cancel(ctx, eventID, nil, now)
}

func (r *EventRepo) cancel(ctx context.Context, eventID uuid.UUID, organizerID *uuid.UUID, now time.Time) (*model.Event, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin event cancellation", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current model.Event
	lockQuery := fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1 AND e.is_public = TRUE FOR UPDATE`, eventProjection)
	args := []any{eventID}
	if organizerID != nil {
		lockQuery = fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1 AND e.organizer_id = $2 AND e.is_public = TRUE FOR UPDATE`, eventProjection)
		args = []any{eventID, *organizerID}
	}
	if err := scanEventInto(tx.QueryRow(ctx, lockQuery, args...), &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock event for cancellation", err)
	}
	if current.Status == model.EventStatusCancelled {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit event cancellation", err)
		}
		return &current, nil
	}
	if _, err := tx.Exec(ctx, "UPDATE events SET status = 'cancelled', updated_at = $2 WHERE id = $1", eventID, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to cancel event", err)
	}
	var cancelled model.Event
	if err := scanEventInto(tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM events e WHERE e.id = $1`, eventProjection), eventID), &cancelled); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read cancelled event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit event cancellation", err)
	}
	return &cancelled, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEventInto(row rowScanner, target *model.Event) error {
	event, err := scanEvent(row)
	if err != nil {
		return err
	}
	*target = *event
	return nil
}

func scanEvent(row rowScanner) (*model.Event, error) {
	var (
		event       model.Event
		calendarID  pgtype.UUID
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
		&calendarID,
		&placeID,
		&latitude,
		&longitude,
		&address,
		&event.City,
		&organizerID,
		&event.IsPublic,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.SoldOut,
	); err != nil {
		return nil, err
	}

	if calendarID.Valid {
		id := uuid.UUID(calendarID.Bytes)
		event.CalendarID = &id
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
