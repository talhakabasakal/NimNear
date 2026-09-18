package eventparticipation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestRSVPLastSeatConcurrencyNeverExceedsCapacity(t *testing.T) {
	pool := testdb.Open(t)
	repo := NewParticipationRepo(pool)
	now := time.Now().UTC()
	capacity := 1
	eventID, userOne, userTwo := createFreeEvent(t, pool, &capacity, now.Add(2*time.Hour))

	results := make(chan error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, userID := range []uuid.UUID{userOne, userTwo} {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			<-start
			_, err := repo.RSVP(context.Background(), eventID, id, now)
			results <- err
		}(userID)
	}
	close(start)
	wg.Wait()
	close(results)

	var successes, soldOut int
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, domainErr.ErrSoldOut) {
			soldOut++
			continue
		}
		t.Fatalf("unexpected concurrent RSVP result: %v", err)
	}
	if successes != 1 || soldOut != 1 {
		t.Fatalf("RSVP vs RSVP successes=%d soldOut=%d, want 1/1", successes, soldOut)
	}
	assertOccupancy(t, pool, eventID, 1)
	assertEffectiveAttendees(t, pool, eventID, 1)
}

func TestRSVPRejectsPaidPastCancelledAndIdempotentRetry(t *testing.T) {
	pool := testdb.Open(t)
	repo := NewParticipationRepo(pool)
	now := time.Now().UTC()
	capacity := 2

	paidID, paidUser, _ := createEvent(t, pool, &capacity, 100000, now.Add(2*time.Hour), "published")
	if _, err := repo.RSVP(context.Background(), paidID, paidUser, now); !errors.Is(err, domainErr.ErrPaidEvent) {
		t.Fatalf("paid RSVP error = %v, want paid event", err)
	}

	pastID, pastUser, _ := createFreeEvent(t, pool, &capacity, now.Add(-time.Minute))
	if _, err := repo.RSVP(context.Background(), pastID, pastUser, now); !errors.Is(err, domainErr.ErrEventPast) {
		t.Fatalf("past RSVP error = %v, want event past", err)
	}

	cancelledID, cancelledUser, organizer := createFreeEvent(t, pool, &capacity, now.Add(2*time.Hour))
	if _, err := pool.Exec(context.Background(), "UPDATE events SET status = 'cancelled' WHERE id = $1", cancelledID); err != nil {
		t.Fatalf("cancel fixture: %v", err)
	}
	if _, err := repo.RSVP(context.Background(), cancelledID, cancelledUser, now); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("cancelled RSVP error = %v, want not found", err)
	}
	_ = organizer

	freeID, user, _ := createFreeEvent(t, pool, &capacity, now.Add(2*time.Hour))
	first, err := repo.RSVP(context.Background(), freeID, user, now)
	if err != nil || !first.Attending || first.AttendeeCount != 1 {
		t.Fatalf("first RSVP = %+v err=%v", first, err)
	}
	again, err := repo.RSVP(context.Background(), freeID, user, now)
	if err != nil || !again.Attending || again.AttendeeCount != 1 {
		t.Fatalf("idempotent RSVP = %+v err=%v", again, err)
	}
	cancelledRSVP, err := repo.Cancel(context.Background(), freeID, user)
	if err != nil || cancelledRSVP.Attending || cancelledRSVP.AttendeeCount != 0 {
		t.Fatalf("un-RSVP = %+v err=%v", cancelledRSVP, err)
	}
	assertEffectiveAttendees(t, pool, freeID, 0)
}

func createFreeEvent(t *testing.T, pool *pgxpool.Pool, capacity *int, end time.Time) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	return createEvent(t, pool, capacity, 0, end, "published")
}

func createEvent(t *testing.T, pool *pgxpool.Pool, capacity *int, price int64, end time.Time, status string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	eventID, userID, organizerID := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{userID, organizerID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@rsvp-test.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, capacity, city, organizer_id, is_public)
		VALUES ($1, 'RSVP test event', '', $2, $3, $4, $5, 'NIM', $6, 'Istanbul', $7, TRUE)`,
		eventID, end.Add(-time.Hour), end, status, price, capacity, organizerID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userID, organizerID)
	})
	return eventID, userID, organizerID
}

func assertOccupancy(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int FROM (
			SELECT user_id FROM event_participants WHERE event_id = $1
			UNION
			SELECT user_id FROM event_purchases WHERE event_id = $1 AND status IN ('confirmed', 'submitted', 'verifying')
			UNION
			SELECT user_id FROM event_purchases WHERE event_id = $1 AND status = 'pending' AND (capacity_hold_expires_at IS NULL OR capacity_hold_expires_at > NOW())
		) active_attendees`, eventID).Scan(&count); err != nil {
		t.Fatalf("occupancy: %v", err)
	}
	if count != want {
		t.Fatalf("occupancy = %d, want %d", count, want)
	}
}

func assertEffectiveAttendees(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int FROM (
			SELECT user_id FROM event_participants WHERE event_id = $1
			UNION
			SELECT user_id FROM event_purchases WHERE event_id = $1 AND status = 'confirmed'
		) effective_attendees`, eventID).Scan(&count); err != nil {
		t.Fatalf("effective attendees: %v", err)
	}
	if count != want {
		t.Fatalf("effective attendees = %d, want %d", count, want)
	}
	var stored int
	if err := pool.QueryRow(context.Background(), "SELECT attendee_count FROM events WHERE id = $1", eventID).Scan(&stored); err != nil {
		t.Fatalf("stored attendee_count: %v", err)
	}
	if stored != want {
		t.Fatalf("stored attendee_count = %d, want %d", stored, want)
	}
}
