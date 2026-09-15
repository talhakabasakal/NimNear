package eventparticipation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// ParticipationRepo implements transactional event RSVP persistence.
type ParticipationRepo struct {
	db *pgxpool.Pool
}

// NewParticipationRepo creates a participation repository.
func NewParticipationRepo(db *pgxpool.Pool) *ParticipationRepo {
	return &ParticipationRepo{db: db}
}

// RSVP locks the event row before checking capacity and inserting the join row.
// The event attendee_count is synchronized from event_participants in the same
// transaction so concurrent requests cannot oversubscribe an event.
func (r *ParticipationRepo) RSVP(ctx context.Context, eventID, userID uuid.UUID, now time.Time) (*model.State, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin RSVP transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	capacity, isPast, isPaid, err := lockEvent(ctx, tx, eventID, now)
	if err != nil {
		return nil, err
	}
	if isPast {
		return nil, domainErr.New(domainErr.ErrEventPast, "event has already ended", nil)
	}
	if isPaid {
		return nil, domainErr.New(domainErr.ErrPaidEvent, "RSVP is available only for free events", nil)
	}

	var attending bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM event_participants
			WHERE event_id = $1 AND user_id = $2
		)`, eventID, userID).Scan(&attending); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to check RSVP", err)
	}

	count, err := syncAttendeeCount(ctx, tx, eventID)
	if err != nil {
		return nil, err
	}
	if attending {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit RSVP", err)
		}
		return state(eventID, true, count, capacity), nil
	}
	if capacity != nil && count >= *capacity {
		return nil, domainErr.New(domainErr.ErrSoldOut, "event is sold out", nil)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO event_participants (event_id, user_id)
		VALUES ($1, $2)`, eventID, userID); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create RSVP", err)
	}

	count, err = syncAttendeeCount(ctx, tx, eventID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit RSVP", err)
	}
	return state(eventID, true, count, capacity), nil
}

// Cancel removes only the supplied user's join row and synchronizes the count.
func (r *ParticipationRepo) Cancel(ctx context.Context, eventID, userID uuid.UUID) (*model.State, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin RSVP cancellation", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	capacity, _, _, err := lockEvent(ctx, tx, eventID, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM event_participants
		WHERE event_id = $1 AND user_id = $2`, eventID, userID); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to cancel RSVP", err)
	}
	count, err := syncAttendeeCount(ctx, tx, eventID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit RSVP cancellation", err)
	}
	return state(eventID, false, count, capacity), nil
}

// GetState reads only the authenticated user's relationship to the public event.
func (r *ParticipationRepo) GetState(ctx context.Context, eventID, userID uuid.UUID) (*model.State, error) {
	var capacity *int
	var count int
	var attending bool
	err := r.db.QueryRow(ctx, `
		SELECT e.capacity,
		       (SELECT COUNT(*)::int FROM event_participants p WHERE p.event_id = e.id),
		       EXISTS (
			       SELECT 1 FROM event_participants p
			       WHERE p.event_id = e.id AND p.user_id = $2
		       )
		FROM events e
		WHERE e.id = $1 AND e.is_public = TRUE AND e.status = 'published'`,
		eventID, userID,
	).Scan(&capacity, &count, &attending)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get RSVP state", err)
	}
	return state(eventID, attending, count, capacity), nil
}

func lockEvent(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, now time.Time) (*int, bool, bool, error) {
	var capacity *int
	var isPast bool
	var isPaid bool
	err := tx.QueryRow(ctx, `
		SELECT capacity, ends_at <= $2, price_lunas <> 0
		FROM events
		WHERE id = $1 AND is_public = TRUE AND status = 'published'
		FOR UPDATE`, eventID, now).Scan(&capacity, &isPast, &isPaid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, false, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, false, false, domainErr.New(domainErr.ErrInternal, "failed to lock event for RSVP", err)
	}
	return capacity, isPast, isPaid, nil
}

func syncAttendeeCount(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `
		UPDATE events
		SET attendee_count = (
			SELECT COUNT(*)::int FROM event_participants
			WHERE event_id = $1
		), updated_at = NOW()
		WHERE id = $1
		RETURNING attendee_count`, eventID).Scan(&count); err != nil {
		return 0, domainErr.New(domainErr.ErrInternal, "failed to synchronize attendee count", err)
	}
	return count, nil
}

func state(eventID uuid.UUID, attending bool, count int, capacity *int) *model.State {
	return &model.State{
		EventID:       eventID,
		Attending:     attending,
		AttendeeCount: count,
		Capacity:      capacity,
	}
}

var _ repository.ParticipationRepository = (*ParticipationRepo)(nil)
