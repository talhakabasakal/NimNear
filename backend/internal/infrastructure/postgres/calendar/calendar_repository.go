package calendar

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// CalendarRepo implements calendar persistence with PostgreSQL.
type CalendarRepo struct {
	db *pgxpool.Pool
}

// NewCalendarRepo creates a PostgreSQL calendar repository.
func NewCalendarRepo(db *pgxpool.Pool) *CalendarRepo {
	return &CalendarRepo{db: db}
}

func (r *CalendarRepo) ListPublic(ctx context.Context, limit int) ([]*model.Calendar, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, image_url, owner_id, visibility, status, created_at, updated_at
		FROM calendars
		WHERE visibility = 'public' AND status = 'active'
		ORDER BY created_at ASC, id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list public calendars", err)
	}
	defer rows.Close()
	return collect(rows)
}

func (r *CalendarRepo) GetPublicByID(ctx context.Context, id uuid.UUID) (*model.Calendar, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, description, image_url, owner_id, visibility, status, created_at, updated_at
		FROM calendars
		WHERE id = $1 AND visibility = 'public' AND status = 'active'`, id)
	calendar, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get public calendar", err)
	}
	return calendar, nil
}

// GetByID returns a calendar for an internal ownership check.
func (r *CalendarRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Calendar, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, description, image_url, owner_id, visibility, status, created_at, updated_at
		FROM calendars
		WHERE id = $1`, id)
	calendar, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get calendar", err)
	}
	return calendar, nil
}

func (r *CalendarRepo) ListOwned(ctx context.Context, ownerID uuid.UUID) ([]*model.Calendar, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, image_url, owner_id, visibility, status, created_at, updated_at
		FROM calendars
		WHERE owner_id = $1
		ORDER BY created_at DESC, id DESC`, ownerID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list owned calendars", err)
	}
	defer rows.Close()
	return collect(rows)
}

func (r *CalendarRepo) ListFollowed(ctx context.Context, userID uuid.UUID) ([]*model.Calendar, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.description, c.image_url, c.owner_id, c.visibility, c.status, c.created_at, c.updated_at
		FROM calendars c
		JOIN calendar_followers f ON f.calendar_id = c.id
		WHERE f.user_id = $1 AND c.visibility = 'public' AND c.status = 'active'
		ORDER BY f.created_at DESC, c.id DESC`, userID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list followed calendars", err)
	}
	defer rows.Close()
	return collect(rows)
}

func (r *CalendarRepo) Create(ctx context.Context, calendar *model.Calendar) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO calendars (id, name, description, image_url, owner_id, visibility, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		calendar.ID, calendar.Name, calendar.Description, calendar.ImageURL, calendar.OwnerID,
		calendar.Visibility, calendar.Status, calendar.CreatedAt, calendar.UpdatedAt)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create calendar", err)
	}
	return nil
}

func (r *CalendarRepo) UpdateOwned(ctx context.Context, calendarID, ownerID uuid.UUID, patch model.Patch, now time.Time) (*model.Calendar, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE calendars
		SET
			name = CASE WHEN $3 THEN $4 ELSE name END,
			description = CASE WHEN $5 THEN $6 ELSE description END,
			image_url = CASE WHEN $7 THEN $8 ELSE image_url END,
			visibility = CASE WHEN $9 THEN $10 ELSE visibility END,
			updated_at = $11
		WHERE id = $1 AND owner_id = $2
		RETURNING id, name, description, image_url, owner_id, visibility, status, created_at, updated_at`,
		calendarID, ownerID,
		patch.NameSet, patch.Name,
		patch.DescriptionSet, patch.Description,
		patch.ImageURLSet, patch.ImageURL,
		patch.VisibilitySet, patch.Visibility,
		now,
	)
	calendar, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to update calendar", err)
	}
	return calendar, nil
}

func (r *CalendarRepo) ArchiveOwned(ctx context.Context, calendarID, ownerID uuid.UUID, now time.Time) (*model.Calendar, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE calendars
		SET status = 'archived', updated_at = $3
		WHERE id = $1 AND owner_id = $2
		RETURNING id, name, description, image_url, owner_id, visibility, status, created_at, updated_at`,
		calendarID, ownerID, now)
	calendar, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to archive calendar", err)
	}
	return calendar, nil
}

func (r *CalendarRepo) Follow(ctx context.Context, calendarID, userID uuid.UUID) error {
	result, err := r.db.Exec(ctx, `
		INSERT INTO calendar_followers (calendar_id, user_id)
		SELECT id, $2
		FROM calendars
		WHERE id = $1 AND visibility = 'public' AND status = 'active'
		ON CONFLICT (calendar_id, user_id) DO NOTHING`, calendarID, userID)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to follow calendar", err)
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM calendar_followers WHERE calendar_id = $1 AND user_id = $2)", calendarID, userID).Scan(&exists); err == nil && exists {
			return nil
		}
		return domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
	}
	return nil
}

func (r *CalendarRepo) Unfollow(ctx context.Context, calendarID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM calendar_followers WHERE calendar_id = $1 AND user_id = $2", calendarID, userID)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to unfollow calendar", err)
	}
	return nil
}

func collect(rows pgx.Rows) ([]*model.Calendar, error) {
	calendars := make([]*model.Calendar, 0)
	for rows.Next() {
		calendar, err := scan(rows)
		if err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan calendar", err)
		}
		calendars = append(calendars, calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to read calendars", err)
	}
	return calendars, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scan(row rowScanner) (*model.Calendar, error) {
	var calendar model.Calendar
	if err := row.Scan(
		&calendar.ID,
		&calendar.Name,
		&calendar.Description,
		&calendar.ImageURL,
		&calendar.OwnerID,
		&calendar.Visibility,
		&calendar.Status,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &calendar, nil
}
