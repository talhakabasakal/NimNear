package event

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventuc "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventrepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	purchasemodel "github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	placemodel "github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/eventparticipation"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/eventpurchase"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/profile"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestP1AAttendanceStatusesAndDedup(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	events := NewEventRepo(pool)
	now := time.Now().UTC()

	t.Run("free RSVP only", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 0, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 1 || got.IsSoldOut() {
			t.Fatalf("free RSVP projection = count %d soldOut %v", got.AttendeeCount, got.IsSoldOut())
		}
		assertCounts(t, pool, fx.eventID, 1, 1)
	})

	t.Run("confirmed paid only", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 100000, now.Add(2*time.Hour))
		insertPurchase(t, pool, fx.eventID, fx.users[0], "confirmed", nil)
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 1 {
			t.Fatalf("confirmed paid attendee_count = %d, want 1", got.AttendeeCount)
		}
		assertCounts(t, pool, fx.eventID, 1, 1)
	})

	t.Run("mixed RSVP and confirmed paid", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 100000, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		insertPurchase(t, pool, fx.eventID, fx.users[1], "confirmed", nil)
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 2 {
			t.Fatalf("mixed attendee_count = %d, want 2", got.AttendeeCount)
		}
		assertCounts(t, pool, fx.eventID, 2, 2)
	})

	t.Run("same user RSVP and confirmed paid counts once", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 100000, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		insertPurchase(t, pool, fx.eventID, fx.users[0], "confirmed", nil)
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 1 {
			t.Fatalf("dedup attendee_count = %d, want 1", got.AttendeeCount)
		}
		assertCounts(t, pool, fx.eventID, 1, 1)
	})

	for _, status := range []string{"pending", "submitted", "verifying", "failed", "expired"} {
		status := status
		t.Run(status+" purchase is not a public attendee", func(t *testing.T) {
			fx := newFixture(t, pool, 5, 100000, now.Add(2*time.Hour))
			hold := now.Add(time.Hour)
			var holdAt *time.Time
			if status == "pending" {
				holdAt = &hold
			}
			insertPurchase(t, pool, fx.eventID, fx.users[0], status, holdAt)
			syncCount(t, pool, fx.eventID)
			got := mustGet(t, events, fx.eventID)
			if got.AttendeeCount != 0 {
				t.Fatalf("%s attendee_count = %d, want 0", status, got.AttendeeCount)
			}
			wantOcc := 0
			if status == "pending" || status == "submitted" || status == "verifying" {
				wantOcc = 1
			}
			assertCounts(t, pool, fx.eventID, 0, wantOcc)
			if got.IsSoldOut() {
				t.Fatalf("%s unexpectedly sold out", status)
			}
		})
	}

	t.Run("recovered expired confirmed is a public attendee", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 1250000, now.Add(2*time.Hour))
		purchases := eventpurchase.NewPurchaseRepo(pool)
		created, err := purchases.Create(ctx, fx.eventID, fx.users[0], now, time.Minute)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		hash := testdb.UniqueHash("f")
		if _, err := purchases.SubmitTransaction(ctx, created.ID, fx.users[0], hash, now, now.Add(-time.Minute)); err != nil {
			t.Fatalf("submit: %v", err)
		}
		expired, err := purchases.ExpireUnresolved(ctx, created.ID, fx.users[0], now, time.Hour)
		if err != nil || expired.Status != purchasemodel.StatusExpired {
			t.Fatalf("expire = %+v err=%v", expired, err)
		}
		assertCounts(t, pool, fx.eventID, 0, 0)
		recovered, err := purchases.ConfirmExpiredRecovery(ctx, created.ID, fx.users[0], now)
		if err != nil || recovered.Status != purchasemodel.StatusConfirmed {
			t.Fatalf("recover = %+v err=%v", recovered, err)
		}
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 1 {
			t.Fatalf("recovered attendee_count = %d, want 1", got.AttendeeCount)
		}
		assertCounts(t, pool, fx.eventID, 1, 1)
	})
}

func TestP1ACapacityHoldsSoldOutAndExpiry(t *testing.T) {
	pool := testdb.Open(t)
	events := NewEventRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	now := time.Now().UTC()
	capacity := 2

	t.Run("case A two attendees sold out", func(t *testing.T) {
		fx := newFixture(t, pool, capacity, 0, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		insertRSVP(t, pool, fx.eventID, fx.users[1])
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 2 || !got.IsSoldOut() {
			t.Fatalf("case A count/soldOut = %d/%v", got.AttendeeCount, got.IsSoldOut())
		}
		assertCounts(t, pool, fx.eventID, 2, 2)
	})

	t.Run("case B attendee plus valid hold exhausts admission", func(t *testing.T) {
		fx := newFixture(t, pool, capacity, 100000, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		hold := now.Add(time.Hour)
		insertPurchase(t, pool, fx.eventID, fx.users[1], "pending", &hold)
		syncCount(t, pool, fx.eventID)
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 1 {
			t.Fatalf("case B attendee_count = %d, want 1 (holds are not public attendees)", got.AttendeeCount)
		}
		if !got.IsSoldOut() {
			t.Fatal("case B want sold out from occupancy including hold")
		}
		assertCounts(t, pool, fx.eventID, 1, 2)
	})

	t.Run("case C expired hold releases capacity", func(t *testing.T) {
		fx := newFixture(t, pool, 1, 500000, now.Add(2*time.Hour))
		stale := now.Add(-time.Hour)
		stalePurchase, err := purchases.Create(context.Background(), fx.eventID, fx.users[0], stale, 10*time.Minute)
		if err != nil {
			t.Fatalf("stale hold: %v", err)
		}
		replacement, err := purchases.Create(context.Background(), fx.eventID, fx.users[1], now, 10*time.Minute)
		if err != nil {
			t.Fatalf("replacement after expiry: %v", err)
		}
		if replacement.ID == stalePurchase.ID {
			t.Fatal("expired hold was not replaced")
		}
		got := mustGet(t, events, fx.eventID)
		if got.AttendeeCount != 0 {
			t.Fatalf("public attendee_count after hold = %d, want 0", got.AttendeeCount)
		}
		if !got.IsSoldOut() {
			t.Fatal("valid replacement hold should exhaust capacity 1")
		}
		assertCounts(t, pool, fx.eventID, 0, 1)
		var expired string
		if err := pool.QueryRow(context.Background(), "SELECT status FROM event_purchases WHERE id = $1", stalePurchase.ID).Scan(&expired); err != nil {
			t.Fatalf("read expired: %v", err)
		}
		if expired != "expired" {
			t.Fatalf("stale status = %q, want expired", expired)
		}
	})

	t.Run("submitted and verifying occupy capacity without public attendance", func(t *testing.T) {
		for _, status := range []string{"submitted", "verifying"} {
			fx := newFixture(t, pool, 1, 100000, now.Add(2*time.Hour))
			insertPurchase(t, pool, fx.eventID, fx.users[0], status, nil)
			syncCount(t, pool, fx.eventID)
			got := mustGet(t, events, fx.eventID)
			if got.AttendeeCount != 0 || !got.IsSoldOut() {
				t.Fatalf("%s count/soldOut = %d/%v, want 0/true", status, got.AttendeeCount, got.IsSoldOut())
			}
			assertCounts(t, pool, fx.eventID, 0, 1)
		}
	})
}

func TestP1ALastSeatConcurrency(t *testing.T) {
	pool := testdb.Open(t)
	rsvps := eventparticipation.NewParticipationRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	now := time.Now().UTC()

	t.Run("RSVP vs purchase last seat on paid event", func(t *testing.T) {
		fx := newFixture(t, pool, 1, 1250000, now.Add(2*time.Hour))
		results := make(chan error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := rsvps.RSVP(context.Background(), fx.eventID, fx.users[0], now)
			results <- err
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := purchases.Create(context.Background(), fx.eventID, fx.users[1], now, time.Minute)
			results <- err
		}()
		close(start)
		wg.Wait()
		close(results)
		var paidRejects, purchaseOK, soldOut int
		for err := range results {
			switch {
			case err == nil:
				purchaseOK++
			case errors.Is(err, domainErr.ErrPaidEvent):
				paidRejects++
			case errors.Is(err, domainErr.ErrSoldOut):
				soldOut++
			default:
				t.Fatalf("unexpected mixed last-seat error: %v", err)
			}
		}
		if paidRejects != 1 {
			t.Fatalf("paid-event RSVP rejects=%d, want 1", paidRejects)
		}
		if purchaseOK+soldOut != 1 {
			t.Fatalf("purchase outcomes ok=%d soldOut=%d, want exactly one", purchaseOK, soldOut)
		}
		assertOccupancyAtMost(t, pool, fx.eventID, 1)
	})

	t.Run("existing RSVP vs concurrent purchases for last seat", func(t *testing.T) {
		fx := newFixture(t, pool, 1, 1250000, now.Add(2*time.Hour))
		insertRSVP(t, pool, fx.eventID, fx.users[0])
		syncCount(t, pool, fx.eventID)
		results := make(chan error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, userID := range []uuid.UUID{fx.users[1], fx.users[2]} {
			wg.Add(1)
			go func(id uuid.UUID) {
				defer wg.Done()
				<-start
				_, err := purchases.Create(context.Background(), fx.eventID, id, now, time.Minute)
				results <- err
			}(userID)
		}
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			if !errors.Is(err, domainErr.ErrSoldOut) {
				t.Fatalf("purchase against RSVP last seat = %v, want sold out", err)
			}
		}
		assertCounts(t, pool, fx.eventID, 1, 1)
	})

	t.Run("purchase vs purchase last seat", func(t *testing.T) {
		fx := newFixture(t, pool, 1, 1250000, now.Add(2*time.Hour))
		results := make(chan error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, userID := range fx.users[:2] {
			wg.Add(1)
			go func(id uuid.UUID) {
				defer wg.Done()
				<-start
				_, err := purchases.Create(context.Background(), fx.eventID, id, now, time.Minute)
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
			t.Fatalf("unexpected purchase vs purchase: %v", err)
		}
		if successes != 1 || soldOut != 1 {
			t.Fatalf("purchase vs purchase successes=%d soldOut=%d", successes, soldOut)
		}
		assertCounts(t, pool, fx.eventID, 0, 1)
	})
}

func TestP1ACapacityEditConcurrency(t *testing.T) {
	pool := testdb.Open(t)
	events := NewEventRepo(pool)
	rsvps := eventparticipation.NewParticipationRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	now := time.Now().UTC()

	t.Run("decrease capacity vs RSVP", func(t *testing.T) {
		fx := newFixture(t, pool, 10, 0, now.Add(2*time.Hour))
		for i := 0; i < 9; i++ {
			insertRSVP(t, pool, fx.eventID, fx.users[i])
		}
		syncCount(t, pool, fx.eventID)
		newCapacity := 9
		start := make(chan struct{})
		var wg sync.WaitGroup
		var patchErr, rsvpErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, patchErr = events.UpdateOwned(context.Background(), fx.eventID, fx.organizerID, eventmodel.Patch{Capacity: &newCapacity, CapacitySet: true}, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, rsvpErr = rsvps.RSVP(context.Background(), fx.eventID, fx.users[9], now)
		}()
		close(start)
		wg.Wait()
		assertSafeCapacityRace(t, pool, fx.eventID, patchErr, rsvpErr)
	})

	t.Run("decrease capacity vs paid purchase", func(t *testing.T) {
		fx := newFixture(t, pool, 10, 1250000, now.Add(2*time.Hour))
		hold := now.Add(time.Hour)
		for i := 0; i < 9; i++ {
			insertPurchase(t, pool, fx.eventID, fx.users[i], "pending", &hold)
		}
		newCapacity := 9
		start := make(chan struct{})
		var wg sync.WaitGroup
		var patchErr, purchaseErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, patchErr = events.UpdateOwned(context.Background(), fx.eventID, fx.organizerID, eventmodel.Patch{Capacity: &newCapacity, CapacitySet: true}, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, purchaseErr = purchases.Create(context.Background(), fx.eventID, fx.users[9], now, time.Minute)
		}()
		close(start)
		wg.Wait()
		assertSafeCapacityRace(t, pool, fx.eventID, patchErr, purchaseErr)
	})
}

func TestP1ACancelConcurrencyAndEvidence(t *testing.T) {
	pool := testdb.Open(t)
	events := NewEventRepo(pool)
	rsvps := eventparticipation.NewParticipationRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	now := time.Now().UTC()

	t.Run("cancel vs RSVP", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 0, now.Add(2*time.Hour))
		start := make(chan struct{})
		var wg sync.WaitGroup
		var cancelErr, rsvpErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, cancelErr = events.CancelOwned(context.Background(), fx.eventID, fx.organizerID, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, rsvpErr = rsvps.RSVP(context.Background(), fx.eventID, fx.users[0], now)
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil {
			t.Fatalf("cancel: %v", cancelErr)
		}
		cancelled := mustGet(t, events, fx.eventID)
		if cancelled.Status != eventmodel.EventStatusCancelled {
			t.Fatalf("status = %s", cancelled.Status)
		}
		if rsvpErr != nil && !errors.Is(rsvpErr, domainErr.ErrNotFound) {
			t.Fatalf("rsvp after/during cancel: %v", rsvpErr)
		}
		if rsvpErr == nil {
			assertEffectiveAttendees(t, pool, fx.eventID, 1)
		} else {
			assertEffectiveAttendees(t, pool, fx.eventID, 0)
		}
		if _, err := rsvps.RSVP(context.Background(), fx.eventID, fx.users[1], now); !errors.Is(err, domainErr.ErrNotFound) {
			t.Fatalf("new RSVP after cancel = %v, want not found", err)
		}
	})

	t.Run("cancel vs purchase create", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 1250000, now.Add(2*time.Hour))
		start := make(chan struct{})
		var wg sync.WaitGroup
		var cancelErr, purchaseErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, cancelErr = events.CancelOwned(context.Background(), fx.eventID, fx.organizerID, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, purchaseErr = purchases.Create(context.Background(), fx.eventID, fx.users[0], now, time.Minute)
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil {
			t.Fatalf("cancel: %v", cancelErr)
		}
		if purchaseErr != nil && !errors.Is(purchaseErr, domainErr.ErrNotFound) {
			t.Fatalf("purchase during cancel: %v", purchaseErr)
		}
		if _, err := purchases.Create(context.Background(), fx.eventID, fx.users[1], now, time.Minute); !errors.Is(err, domainErr.ErrNotFound) {
			t.Fatalf("new purchase after cancel = %v, want not found", err)
		}
	})

	t.Run("cancel vs submit preserves hash when submit wins", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 1250000, now.Add(2*time.Hour))
		created, err := purchases.Create(context.Background(), fx.eventID, fx.users[0], now, time.Minute)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		hash := testdb.UniqueHash("c")
		start := make(chan struct{})
		var wg sync.WaitGroup
		var cancelErr, submitErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, cancelErr = events.CancelOwned(context.Background(), fx.eventID, fx.organizerID, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, submitErr = purchases.SubmitTransaction(context.Background(), created.ID, fx.users[0], hash, now, now.Add(time.Hour))
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil {
			t.Fatalf("cancel: %v", cancelErr)
		}
		owned, err := purchases.GetOwned(context.Background(), created.ID, fx.users[0])
		if err != nil {
			t.Fatalf("owned after cancel/submit: %v", err)
		}
		if submitErr == nil {
			if owned.TransactionHash == nil || *owned.TransactionHash != hash || owned.Status != purchasemodel.StatusSubmitted {
				t.Fatalf("submit won but evidence missing: %+v", owned)
			}
			var consumed int
			if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM consumed_nimiq_transactions WHERE transaction_hash = $1 AND domain_id = $2", hash, created.ID).Scan(&consumed); err != nil || consumed != 1 {
				t.Fatalf("consumed evidence = %d err=%v", consumed, err)
			}
		} else if domainErr.ErrorCode(submitErr) != "event_cancelled" && !errors.Is(submitErr, domainErr.ErrConflict) {
			t.Fatalf("submit during cancel: %v", submitErr)
		}
		if owned.ID != created.ID {
			t.Fatal("purchase row deleted by cancellation")
		}
		mustGet(t, events, fx.eventID)
	})

	t.Run("cancel vs verification confirm does not delete evidence", func(t *testing.T) {
		fx := newFixture(t, pool, 5, 1250000, now.Add(2*time.Hour))
		created, err := purchases.Create(context.Background(), fx.eventID, fx.users[0], now, time.Minute)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		hash := testdb.UniqueHash("d")
		submitted, err := purchases.SubmitTransaction(context.Background(), created.ID, fx.users[0], hash, now, now.Add(time.Hour))
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		var cancelErr, verifyErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, cancelErr = events.CancelOwned(context.Background(), fx.eventID, fx.organizerID, now)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, verifyErr = purchases.SetVerificationState(context.Background(), submitted.ID, fx.users[0], purchasemodel.StatusConfirmed, now)
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil {
			t.Fatalf("cancel: %v", cancelErr)
		}
		if verifyErr != nil {
			t.Fatalf("verification during cancel: %v", verifyErr)
		}
		owned, err := purchases.GetOwned(context.Background(), submitted.ID, fx.users[0])
		if err != nil {
			t.Fatalf("owned: %v", err)
		}
		if owned.TransactionHash == nil || *owned.TransactionHash != hash {
			t.Fatalf("hash lost: %+v", owned)
		}
		var consumed int
		if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM consumed_nimiq_transactions WHERE transaction_hash = $1", hash).Scan(&consumed); err != nil || consumed != 1 {
			t.Fatalf("consumed = %d err=%v", consumed, err)
		}
		cancelled := mustGet(t, events, fx.eventID)
		if cancelled.Status != eventmodel.EventStatusCancelled {
			t.Fatalf("status = %s", cancelled.Status)
		}
	})
}

func TestP1AProfileHistoryAndAttendeeDrift(t *testing.T) {
	pool := testdb.Open(t)
	events := NewEventRepo(pool)
	rsvps := eventparticipation.NewParticipationRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	profiles := profile.NewProfileRepo(pool)
	now := time.Now().UTC()

	user := insertUser(t, pool, "profile-history")
	free := newFixtureWithOrganizerUsers(t, pool, 5, 0, now.Add(3*time.Hour), user)
	paid := newFixtureWithOrganizerUsers(t, pool, 5, 1250000, now.Add(4*time.Hour), user)
	pendingEvent := newFixtureWithOrganizerUsers(t, pool, 5, 1250000, now.Add(5*time.Hour), user)
	failedEvent := newFixtureWithOrganizerUsers(t, pool, 5, 1250000, now.Add(6*time.Hour), user)
	expiredEvent := newFixtureWithOrganizerUsers(t, pool, 5, 1250000, now.Add(7*time.Hour), user)
	dup := newFixtureWithOrganizerUsers(t, pool, 5, 1250000, now.Add(8*time.Hour), user)

	if _, err := rsvps.RSVP(context.Background(), free.eventID, user, now); err != nil {
		t.Fatalf("free RSVP: %v", err)
	}
	confirmed, err := purchases.Create(context.Background(), paid.eventID, user, now, time.Minute)
	if err != nil {
		t.Fatalf("paid create: %v", err)
	}
	if _, err := purchases.SubmitTransaction(context.Background(), confirmed.ID, user, testdb.UniqueHash("a"), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("paid submit: %v", err)
	}
	if _, err := purchases.SetVerificationState(context.Background(), confirmed.ID, user, purchasemodel.StatusConfirmed, now); err != nil {
		t.Fatalf("paid confirm: %v", err)
	}
	if _, err := purchases.Create(context.Background(), pendingEvent.eventID, user, now, time.Minute); err != nil {
		t.Fatalf("pending: %v", err)
	}
	failedPurchase, err := purchases.Create(context.Background(), failedEvent.eventID, user, now, time.Minute)
	if err != nil {
		t.Fatalf("failed create: %v", err)
	}
	if _, err := purchases.SubmitTransaction(context.Background(), failedPurchase.ID, user, testdb.UniqueHash("b"), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("failed submit: %v", err)
	}
	if _, err := purchases.SetVerificationState(context.Background(), failedPurchase.ID, user, purchasemodel.StatusFailed, now); err != nil {
		t.Fatalf("failed confirm state: %v", err)
	}
	expiredPurchase, err := purchases.Create(context.Background(), expiredEvent.eventID, user, now, time.Minute)
	if err != nil {
		t.Fatalf("expired create: %v", err)
	}
	if _, err := purchases.SubmitTransaction(context.Background(), expiredPurchase.ID, user, testdb.UniqueHash("e"), now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("expired submit: %v", err)
	}
	if _, err := purchases.ExpireUnresolved(context.Background(), expiredPurchase.ID, user, now, time.Hour); err != nil {
		t.Fatalf("expire: %v", err)
	}
	insertRSVP(t, pool, dup.eventID, user)
	insertPurchase(t, pool, dup.eventID, user, "confirmed", nil)
	syncCount(t, pool, dup.eventID)

	attended, err := events.ListPublicByAttendee(context.Background(), user)
	if err != nil {
		t.Fatalf("attended list: %v", err)
	}
	ids := map[uuid.UUID]int{}
	for _, event := range attended {
		ids[event.ID]++
	}
	if ids[free.eventID] != 1 || ids[paid.eventID] != 1 || ids[dup.eventID] != 1 {
		t.Fatalf("attended ids = %v", ids)
	}
	if ids[pendingEvent.eventID] != 0 || ids[failedEvent.eventID] != 0 || ids[expiredEvent.eventID] != 0 {
		t.Fatalf("non-confirmed leaked into attended: %v", ids)
	}
	if len(attended) != 3 {
		t.Fatalf("attended count = %d, want 3 unique events", len(attended))
	}
	if !attended[0].StartsAt.Before(attended[1].StartsAt) || !attended[1].StartsAt.Before(attended[2].StartsAt) {
		t.Fatalf("attended ordering not by starts_at: %v %v %v", attended[0].StartsAt, attended[1].StartsAt, attended[2].StartsAt)
	}

	public, err := profiles.GetPublic(context.Background(), user)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if public.AttendedEventCount != 3 {
		t.Fatalf("profile attended count = %d, want 3", public.AttendedEventCount)
	}

	drift := newFixture(t, pool, 10, 1250000, now.Add(2*time.Hour))
	freeDrift := newFixture(t, pool, 10, 0, now.Add(2*time.Hour))
	state, err := rsvps.RSVP(context.Background(), freeDrift.eventID, drift.users[0], now)
	if err != nil || state.AttendeeCount != 1 {
		t.Fatalf("drift RSVP = %+v err=%v", state, err)
	}
	assertCounts(t, pool, freeDrift.eventID, 1, 1)
	cancelled, err := rsvps.Cancel(context.Background(), freeDrift.eventID, drift.users[0])
	if err != nil || cancelled.AttendeeCount != 0 {
		t.Fatalf("drift un-RSVP = %+v err=%v", cancelled, err)
	}
	assertCounts(t, pool, freeDrift.eventID, 0, 0)

	hold, err := purchases.Create(context.Background(), drift.eventID, drift.users[0], now, time.Minute)
	if err != nil {
		t.Fatalf("drift hold: %v", err)
	}
	got := mustGet(t, events, drift.eventID)
	if got.AttendeeCount != 0 {
		t.Fatalf("hold changed public count to %d", got.AttendeeCount)
	}
	assertCounts(t, pool, drift.eventID, 0, 1)
	if _, err := purchases.SubmitTransaction(context.Background(), hold.ID, drift.users[0], testdb.UniqueHash("1"), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("drift submit: %v", err)
	}
	confirmedHold, err := purchases.SetVerificationState(context.Background(), hold.ID, drift.users[0], purchasemodel.StatusConfirmed, now)
	if err != nil || confirmedHold.Status != purchasemodel.StatusConfirmed {
		t.Fatalf("drift confirm = %+v err=%v", confirmedHold, err)
	}
	assertCounts(t, pool, drift.eventID, 1, 1)

	failedHold, err := purchases.Create(context.Background(), drift.eventID, drift.users[1], now, time.Minute)
	if err != nil {
		t.Fatalf("drift failed create: %v", err)
	}
	if _, err := purchases.SubmitTransaction(context.Background(), failedHold.ID, drift.users[1], testdb.UniqueHash("2"), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("drift failed submit: %v", err)
	}
	if _, err := purchases.SetVerificationState(context.Background(), failedHold.ID, drift.users[1], purchasemodel.StatusFailed, now); err != nil {
		t.Fatalf("drift fail: %v", err)
	}
	assertCounts(t, pool, drift.eventID, 1, 1)

	exp, err := purchases.Create(context.Background(), drift.eventID, drift.users[2], now, time.Minute)
	if err != nil {
		t.Fatalf("drift expire create: %v", err)
	}
	if _, err := purchases.SubmitTransaction(context.Background(), exp.ID, drift.users[2], testdb.UniqueHash("3"), now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("drift expire submit: %v", err)
	}
	if _, err := purchases.ExpireUnresolved(context.Background(), exp.ID, drift.users[2], now, time.Hour); err != nil {
		t.Fatalf("drift expire: %v", err)
	}
	assertCounts(t, pool, drift.eventID, 1, 1)
	if _, err := purchases.ConfirmExpiredRecovery(context.Background(), exp.ID, drift.users[2], now); err != nil {
		t.Fatalf("drift recover: %v", err)
	}
	assertCounts(t, pool, drift.eventID, 2, 2)
	proj := mustGet(t, events, drift.eventID)
	if proj.AttendeeCount != 2 {
		t.Fatalf("product attendee_count drifted to %d", proj.AttendeeCount)
	}
}

func TestP1AEventUpdateAndCancel(t *testing.T) {
	pool := testdb.Open(t)
	events := NewEventRepo(pool)
	places := place.NewPlaceRepo(pool)
	uc := eventuc.NewEventUseCaseWithAssociations(events, places, nil)
	rsvps := eventparticipation.NewParticipationRepo(pool)
	purchases := eventpurchase.NewPurchaseRepo(pool)
	now := time.Now().UTC()

	activePlace := &placemodel.Place{ID: uuid.New(), Name: "Active Hall", Latitude: 41.0, Longitude: 29.0, Category: "venue", IsActive: true}
	if err := places.Create(context.Background(), activePlace); err != nil {
		t.Fatalf("create place: %v", err)
	}
	inactivePlace := &placemodel.Place{ID: uuid.New(), Name: "Inactive Hall", Latitude: 41.1, Longitude: 29.1, Category: "venue", IsActive: true}
	if err := places.Create(context.Background(), inactivePlace); err != nil {
		t.Fatalf("create inactive place: %v", err)
	}
	inactivePlace.IsActive = false
	if err := places.Update(context.Background(), inactivePlace); err != nil {
		t.Fatalf("disable place: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM places WHERE id = ANY($1)", []uuid.UUID{activePlace.ID, inactivePlace.ID})
	})

	fx := newFixture(t, pool, 10, 0, now.Add(2*time.Hour))
	updated, err := events.UpdateOwned(context.Background(), fx.eventID, fx.organizerID, eventmodel.Patch{Title: "Owner title", TitleSet: true}, now)
	if err != nil || updated.Title != "Owner title" {
		t.Fatalf("owner update = %+v err=%v", updated, err)
	}
	if _, err := events.UpdateOwned(context.Background(), fx.eventID, fx.users[0], eventmodel.Patch{Title: "Hijack", TitleSet: true}, now); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("non-owner update = %v, want not found", err)
	}

	insertRSVP(t, pool, fx.eventID, fx.users[0])
	insertRSVP(t, pool, fx.eventID, fx.users[1])
	syncCount(t, pool, fx.eventID)
	tooLow := 1
	if _, err := events.UpdateOwned(context.Background(), fx.eventID, fx.organizerID, eventmodel.Patch{Capacity: &tooLow, CapacitySet: true}, now); !errors.Is(err, domainErr.ErrConflict) {
		t.Fatalf("capacity below occupancy = %v, want conflict", err)
	}

	if _, err := uc.Update(context.Background(), fx.organizerID, fx.eventID, eventdto.UpdateEventRequest{PlaceID: eventdto.OptionalUUID{Set: true, Value: &inactivePlace.ID}}); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("inactive place = %v, want not found", err)
	}
	missing := uuid.New()
	if _, err := uc.Update(context.Background(), fx.organizerID, fx.eventID, eventdto.UpdateEventRequest{PlaceID: eventdto.OptionalUUID{Set: true, Value: &missing}}); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("missing place = %v, want not found", err)
	}
	if _, err := uc.Update(context.Background(), fx.organizerID, fx.eventID, eventdto.UpdateEventRequest{PlaceID: eventdto.OptionalUUID{Set: true, Value: &activePlace.ID}}); err != nil {
		t.Fatalf("valid place update: %v", err)
	}

	paid := newFixture(t, pool, 10, 1250000, now.Add(2*time.Hour))
	if err := pool.QueryRow(context.Background(), "UPDATE events SET place_id = $2 WHERE id = $1 RETURNING id", paid.eventID, activePlace.ID).Scan(&paid.eventID); err != nil {
		t.Fatalf("set paid place: %v", err)
	}
	hold := now.Add(time.Hour)
	insertPurchase(t, pool, paid.eventID, paid.users[0], "confirmed", &hold)
	otherPlace := uuid.New()
	if _, err := events.UpdateOwned(context.Background(), paid.eventID, paid.organizerID, eventmodel.Patch{PlaceID: &otherPlace, PlaceIDSet: true}, now); !errors.Is(err, domainErr.ErrConflict) {
		t.Fatalf("paid location edit = %v, want conflict", err)
	}
	allowed, err := events.UpdateOwned(context.Background(), paid.eventID, paid.organizerID, eventmodel.Patch{Title: "Allowed paid title", TitleSet: true}, now)
	if err != nil || allowed.Title != "Allowed paid title" {
		t.Fatalf("allowed paid edit = %+v err=%v", allowed, err)
	}

	listed, err := events.ListPublic(context.Background(), eventrepo.ListFilter{City: fx.city, Limit: 100})
	if err != nil {
		t.Fatalf("list before cancel: %v", err)
	}
	if !containsEvent(listed, fx.eventID) {
		t.Fatal("published event missing from discovery")
	}

	if _, err := events.CancelOwned(context.Background(), fx.eventID, fx.users[0], now); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("non-owner cancel = %v, want not found", err)
	}
	cancelled, err := events.CancelOwned(context.Background(), fx.eventID, fx.organizerID, now)
	if err != nil || cancelled.Status != eventmodel.EventStatusCancelled {
		t.Fatalf("owner cancel = %+v err=%v", cancelled, err)
	}
	after, err := events.ListPublic(context.Background(), eventrepo.ListFilter{City: fx.city, Limit: 100})
	if err != nil {
		t.Fatalf("list after cancel: %v", err)
	}
	if containsEvent(after, fx.eventID) {
		t.Fatal("cancelled event leaked into discovery")
	}
	direct, err := events.GetPublicByID(context.Background(), fx.eventID)
	if err != nil || direct.Status != eventmodel.EventStatusCancelled {
		t.Fatalf("direct cancelled detail = %+v err=%v", direct, err)
	}
	if _, err := rsvps.RSVP(context.Background(), fx.eventID, fx.users[3], now); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("RSVP cancelled = %v, want not found", err)
	}
	if _, err := purchases.Create(context.Background(), paid.eventID, paid.users[1], now, time.Minute); err != nil {
		t.Fatalf("control purchase on live paid event: %v", err)
	}
	if _, err := events.CancelOwned(context.Background(), paid.eventID, paid.organizerID, now); err != nil {
		t.Fatalf("cancel paid: %v", err)
	}
	if _, err := purchases.Create(context.Background(), paid.eventID, paid.users[2], now, time.Minute); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("purchase cancelled = %v, want not found", err)
	}
	var purchaseCount int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM event_purchases WHERE event_id = $1", paid.eventID).Scan(&purchaseCount); err != nil || purchaseCount == 0 {
		t.Fatalf("paid evidence missing after cancel: count=%d err=%v", purchaseCount, err)
	}
	var rsvpCount int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM event_participants WHERE event_id = $1", fx.eventID).Scan(&rsvpCount); err != nil || rsvpCount != 2 {
		t.Fatalf("RSVP evidence missing after cancel: count=%d err=%v", rsvpCount, err)
	}
}

type p1aFixture struct {
	eventID     uuid.UUID
	organizerID uuid.UUID
	city        string
	users       []uuid.UUID
}

func newFixture(t *testing.T, pool *pgxpool.Pool, capacity int, price int64, end time.Time) p1aFixture {
	t.Helper()
	users := make([]uuid.UUID, 10)
	for i := range users {
		users[i] = insertUser(t, pool, "p1a")
	}
	return newFixtureWithUsers(t, pool, capacity, price, end, users)
}

func newFixtureWithOrganizerUsers(t *testing.T, pool *pgxpool.Pool, capacity int, price int64, end time.Time, attendee uuid.UUID) p1aFixture {
	t.Helper()
	users := make([]uuid.UUID, 10)
	users[0] = attendee
	for i := 1; i < 10; i++ {
		users[i] = insertUser(t, pool, "p1a-extra")
	}
	return newFixtureWithUsers(t, pool, capacity, price, end, users)
}

func newFixtureWithUsers(t *testing.T, pool *pgxpool.Pool, capacity int, price int64, end time.Time, users []uuid.UUID) p1aFixture {
	t.Helper()
	ctx := context.Background()
	organizerID := insertUser(t, pool, "p1a-org")
	eventID := uuid.New()
	city := "p1a-" + eventID.String()[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, capacity, city, organizer_id, is_public)
		VALUES ($1, $2, '', $3, $4, 'published', $5, 'NIM', $6, $7, $8, TRUE)`,
		eventID, eventID.String(), end.Add(-time.Hour), end, price, capacity, city, organizerID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM consumed_nimiq_transactions
			WHERE domain_type = 'event_purchase'
			  AND domain_id IN (SELECT id FROM event_purchases WHERE event_id = $1)`, eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
	})
	return p1aFixture{eventID: eventID, organizerID: organizerID, city: city, users: users}
}

func insertUser(t *testing.T, pool *pgxpool.Pool, suffix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@"+suffix+".local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", id)
	})
	return id
}

func insertRSVP(t *testing.T, pool *pgxpool.Pool, eventID, userID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "INSERT INTO event_participants (event_id, user_id) VALUES ($1, $2)", eventID, userID); err != nil {
		t.Fatalf("insert RSVP: %v", err)
	}
}

func insertPurchase(t *testing.T, pool *pgxpool.Pool, eventID, userID uuid.UUID, status string, hold *time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_purchases (event_id, user_id, amount_lunas, status, capacity_hold_expires_at)
		VALUES ($1, $2, 100000, $3, $4)`, eventID, userID, status, hold); err != nil {
		t.Fatalf("insert purchase %s: %v", status, err)
	}
}

func syncCount(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		UPDATE events SET attendee_count = (
			SELECT COUNT(*)::int FROM (
				SELECT user_id FROM event_participants WHERE event_id = $1
				UNION
				SELECT user_id FROM event_purchases WHERE event_id = $1 AND status = 'confirmed'
			) effective_attendees
		) WHERE id = $1`, eventID); err != nil {
		t.Fatalf("sync attendee_count: %v", err)
	}
}

func mustGet(t *testing.T, repo *EventRepo, eventID uuid.UUID) *eventmodel.Event {
	t.Helper()
	got, err := repo.GetPublicByID(context.Background(), eventID)
	if err != nil {
		t.Fatalf("GetPublicByID: %v", err)
	}
	return got
}

func assertCounts(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, wantAttendees, wantOccupancy int) {
	t.Helper()
	assertEffectiveAttendees(t, pool, eventID, wantAttendees)
	assertOccupancy(t, pool, eventID, wantOccupancy)
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

func assertOccupancy(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, want int) {
	t.Helper()
	got := occupancy(t, pool, eventID)
	if got != want {
		t.Fatalf("occupancy = %d, want %d", got, want)
	}
}

func assertOccupancyAtMost(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, max int) {
	t.Helper()
	got := occupancy(t, pool, eventID)
	if got > max {
		t.Fatalf("occupancy = %d, exceeds capacity %d", got, max)
	}
}

func occupancy(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) int {
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
	return count
}

func assertSafeCapacityRace(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, mutationErr, joinErr error) {
	t.Helper()
	if mutationErr != nil && !errors.Is(mutationErr, domainErr.ErrConflict) {
		t.Fatalf("capacity mutation: %v", mutationErr)
	}
	if joinErr != nil && !errors.Is(joinErr, domainErr.ErrSoldOut) {
		t.Fatalf("join during capacity mutation: %v", joinErr)
	}
	if mutationErr == nil && joinErr == nil {
		t.Fatal("both capacity decrease and new participant succeeded; occupancy may exceed new capacity")
	}
	var capacity int
	if err := pool.QueryRow(context.Background(), "SELECT capacity FROM events WHERE id = $1", eventID).Scan(&capacity); err != nil {
		t.Fatalf("read capacity: %v", err)
	}
	got := occupancy(t, pool, eventID)
	if got > capacity {
		t.Fatalf("occupancy %d exceeds capacity %d after race", got, capacity)
	}
}

func containsEvent(events []*eventmodel.Event, id uuid.UUID) bool {
	for _, event := range events {
		if event.ID == id {
			return true
		}
	}
	return false
}
