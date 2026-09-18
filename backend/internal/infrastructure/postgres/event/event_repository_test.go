package event

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
)

func TestEventProjectionIncludesConfirmedPaidAttendanceAndActiveHolds(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	repo := NewEventRepo(pool)
	eventID, organizerID, attendeeID, pendingID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	for _, id := range []uuid.UUID{organizerID, attendeeID, pendingID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@event-projection-test.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	capacity := 1
	if _, err := pool.Exec(ctx, `INSERT INTO events (id, title, starts_at, ends_at, status, price_lunas, currency, capacity, city, organizer_id, is_public) VALUES ($1, 'Projection event', $2, $3, 'published', 100000, 'NIM', $4, 'Example City', $5, TRUE)`, eventID, now.Add(time.Hour), now.Add(2*time.Hour), capacity, organizerID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO event_participants (event_id, user_id) VALUES ($1, $2)", eventID, attendeeID); err != nil {
		t.Fatalf("insert RSVP: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO event_purchases (event_id, user_id, amount_lunas, status) VALUES ($1, $2, 100000, 'confirmed')", eventID, attendeeID); err != nil {
		t.Fatalf("insert confirmed purchase: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO event_purchases (event_id, user_id, amount_lunas, status, capacity_hold_expires_at) VALUES ($1, $2, 100000, 'pending', $3)", eventID, pendingID, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert pending hold: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{organizerID, attendeeID, pendingID})
	})

	event, err := repo.GetPublicByID(ctx, eventID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if event.AttendeeCount != 1 || !event.IsSoldOut() {
		t.Fatalf("projection count/sold-out = %d/%v, want 1/true", event.AttendeeCount, event.IsSoldOut())
	}
	attended, err := repo.ListPublicByAttendee(ctx, attendeeID)
	if err != nil || len(attended) != 1 {
		t.Fatalf("paid attendee history = %d, err=%v; want one event", len(attended), err)
	}

	updated, err := repo.UpdateOwned(ctx, eventID, organizerID, eventmodel.Patch{Title: "Updated projection event", TitleSet: true}, now)
	if err != nil || updated.Title != "Updated projection event" {
		t.Fatalf("update = %+v, err=%v", updated, err)
	}
	cancelled, err := repo.CancelOwned(ctx, eventID, organizerID, now)
	if err != nil || cancelled.Status != eventmodel.EventStatusCancelled {
		t.Fatalf("cancel = %+v, err=%v", cancelled, err)
	}
	direct, err := repo.GetPublicByID(ctx, eventID)
	if err != nil || direct.Status != eventmodel.EventStatusCancelled {
		t.Fatalf("cancelled detail = %+v, err=%v", direct, err)
	}
}

func TestEventOperatorCancelPreservesPurchases(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	repo := NewEventRepo(pool)
	eventID, organizerID, attendeeID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	for _, id := range []uuid.UUID{organizerID, attendeeID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@event-operator-cancel.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO events (id, title, starts_at, ends_at, status, price_lunas, currency, city, organizer_id, is_public) VALUES ($1, 'Operator event', $2, $3, 'published', 100000, 'NIM', 'Example City', $4, TRUE)`, eventID, now.Add(time.Hour), now.Add(2*time.Hour), organizerID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO event_purchases (event_id, user_id, amount_lunas, status, transaction_hash) VALUES ($1, $2, 100000, 'confirmed', $3)", eventID, attendeeID, "cc"+eventID.String()+attendeeID.String()[:8]); err != nil {
		t.Fatalf("insert purchase: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{organizerID, attendeeID})
	})
	cancelled, err := repo.Cancel(ctx, eventID, now)
	if err != nil || cancelled.Status != eventmodel.EventStatusCancelled {
		t.Fatalf("operator cancel = %+v err=%v", cancelled, err)
	}
	var purchaseCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM event_purchases WHERE event_id = $1 AND status = 'confirmed'", eventID).Scan(&purchaseCount); err != nil || purchaseCount != 1 {
		t.Fatalf("purchase evidence = %d err=%v", purchaseCount, err)
	}
}
