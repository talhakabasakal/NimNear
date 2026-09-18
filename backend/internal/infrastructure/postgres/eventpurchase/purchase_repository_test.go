package eventpurchase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func testPurchaseDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.Open(t)
}

func createPurchaseFixture(t *testing.T, pool *pgxpool.Pool, capacity *int, price int64, end time.Time) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	eventID, userOne, userTwo := uuid.New(), uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{userOne, userTwo} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", userID, userID.String()+"@purchase-test.local"); err != nil {
			t.Fatalf("insert fixture user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, capacity, city, is_public)
		VALUES ($1, 'Purchase test event', '', $2, $3, 'published', $4, 'NIM', $5, 'Istanbul', TRUE)`,
		eventID, end.Add(-time.Hour), end, price, capacity); err != nil {
		t.Fatalf("insert fixture event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM consumed_nimiq_transactions
			WHERE domain_type = 'event_purchase'
			  AND domain_id IN (SELECT id FROM event_purchases WHERE event_id = $1)`, eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userOne, userTwo)
	})
	return eventID, userOne, userTwo
}

func TestPurchaseRepositoryConcurrencyAndLazyExpiration(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	capacity := 1
	eventID, userOne, userTwo := createPurchaseFixture(t, pool, &capacity, 1250000, now.Add(2*time.Hour))

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, userID := range []uuid.UUID{userOne, userTwo} {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			_, err := repo.Create(context.Background(), eventID, id, now, 10*time.Minute)
			results <- err
		}(userID)
	}
	wg.Wait()
	close(results)

	var successes, soldOut int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, domainErr.ErrSoldOut) {
			soldOut++
		} else {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if successes != 1 || soldOut != 1 {
		t.Fatalf("concurrency results = successes %d, sold out %d", successes, soldOut)
	}

	first, err := repo.Create(context.Background(), eventID, userOne, now, 10*time.Minute)
	if errors.Is(err, domainErr.ErrSoldOut) {
		first, err = repo.Create(context.Background(), eventID, userTwo, now, 10*time.Minute)
	}
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	second, err := repo.Create(context.Background(), eventID, first.UserID, now, 10*time.Minute)
	if err != nil {
		t.Fatalf("second idempotent retry: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent retry IDs differ: %s vs %s", first.ID, second.ID)
	}

	expiryEvent, expiryUserOne, expiryUserTwo := createPurchaseFixture(t, pool, &capacity, 500000, now.Add(2*time.Hour))
	stale := now.Add(-time.Hour)
	stalePurchase, err := repo.Create(context.Background(), expiryEvent, expiryUserOne, stale, 10*time.Minute)
	if err != nil {
		t.Fatalf("create stale hold: %v", err)
	}
	if stalePurchase.CapacityHoldExpiresAt == nil {
		t.Fatal("expected capacity hold expiry")
	}
	replacement, err := repo.Create(context.Background(), expiryEvent, expiryUserTwo, now, 10*time.Minute)
	if err != nil {
		t.Fatalf("create after lazy expiration: %v", err)
	}
	if replacement.ID == stalePurchase.ID || replacement.Status != "pending" {
		t.Fatalf("unexpected replacement purchase: %#v", replacement)
	}
	var expired string
	if err := pool.QueryRow(context.Background(), "SELECT status FROM event_purchases WHERE id = $1", stalePurchase.ID).Scan(&expired); err != nil {
		t.Fatalf("read expired hold: %v", err)
	}
	if expired != "expired" {
		t.Fatalf("stale hold status = %q, want expired", expired)
	}
}

func TestPurchaseRepositoryRejectsFreePastAndMissingEvents(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()

	freeEvent, freeUser, _ := createPurchaseFixture(t, pool, nil, 0, now.Add(2*time.Hour))
	if _, err := repo.Create(context.Background(), freeEvent, freeUser, now, time.Minute); !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("free event error = %v", err)
	}
	pastEvent, pastUser, _ := createPurchaseFixture(t, pool, nil, 100000, now.Add(-time.Minute))
	if _, err := repo.Create(context.Background(), pastEvent, pastUser, now, time.Minute); !errors.Is(err, domainErr.ErrEventPast) {
		t.Fatalf("past event error = %v", err)
	}
	if _, err := repo.Create(context.Background(), uuid.New(), freeUser, now, time.Minute); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("missing event error = %v", err)
	}
}

func TestPurchaseRepositoryOwnerScopedRead(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	eventID, userOne, userTwo := createPurchaseFixture(t, pool, nil, 100000, now.Add(2*time.Hour))
	purchase, err := repo.Create(context.Background(), eventID, userOne, now, time.Minute)
	if err != nil {
		t.Fatalf("create purchase: %v", err)
	}
	owned, err := repo.GetOwned(context.Background(), purchase.ID, userOne)
	if err != nil || owned.ID != purchase.ID {
		t.Fatalf("owned read = %#v, %v", owned, err)
	}
	if _, err := repo.GetOwned(context.Background(), purchase.ID, userTwo); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("foreign read error = %v", err)
	}
}

func TestPurchaseRepositoryVerificationClaimIsExclusiveAndTerminalStateWins(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	eventID, userOne, _ := createPurchaseFixture(t, pool, nil, 1250000, now.Add(2*time.Hour))
	purchase, err := repo.Create(context.Background(), eventID, userOne, now, 10*time.Minute)
	if err != nil {
		t.Fatalf("create purchase: %v", err)
	}
	deadline := now.Add(time.Hour)
	purchase, err = repo.SubmitTransaction(context.Background(), purchase.ID, userOne, testdb.UniqueHash("a"), now, deadline)
	if err != nil {
		t.Fatalf("submit transaction: %v", err)
	}

	claims := make(chan *model.Purchase, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, claimErr := repo.ClaimVerification(context.Background(), purchase.ID, userOne, now, time.Minute, time.Minute, false)
			if claimErr != nil {
				t.Errorf("claim verification: %v", claimErr)
				return
			}
			claims <- claimed
		}()
	}
	wg.Wait()
	close(claims)
	var claimedCount int
	for claim := range claims {
		if claim != nil {
			claimedCount++
		}
	}
	if claimedCount != 1 {
		t.Fatalf("claimed count = %d, want exactly one", claimedCount)
	}

	candidates, err := repo.ListReconciliationCandidates(context.Background(), now, time.Minute, time.Hour, 10)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.ID == purchase.ID {
			t.Fatal("leased purchase appeared as a reconciliation candidate")
		}
	}

	confirmed, err := repo.SetVerificationState(context.Background(), purchase.ID, userOne, model.StatusConfirmed, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("confirm purchase: %v", err)
	}
	if confirmed.Status != model.StatusConfirmed {
		t.Fatalf("status = %q, want confirmed", confirmed.Status)
	}
	unchanged, err := repo.ExpireUnresolved(context.Background(), purchase.ID, userOne, now.Add(2*time.Hour), time.Hour)
	if err != nil {
		t.Fatalf("expire terminal purchase: %v", err)
	}
	if unchanged.Status != model.StatusConfirmed {
		t.Fatalf("terminal status resurrected/changed to %q", unchanged.Status)
	}
}

func TestPurchaseRepositoryRSVPRemainsCapacityActive(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	capacity := 1
	eventID, rsvpUser, otherUser := createPurchaseFixture(t, pool, &capacity, 1250000, now.Add(2*time.Hour))
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_participants (event_id, user_id) VALUES ($1, $2)`, eventID, rsvpUser); err != nil {
		t.Fatalf("insert RSVP fixture: %v", err)
	}
	if _, err := repo.Create(context.Background(), eventID, otherUser, now, time.Minute); !errors.Is(err, domainErr.ErrSoldOut) {
		t.Fatalf("purchase after RSVP error = %v, want sold out", err)
	}
}

func TestPurchaseRepositoryTransactionHashSubmissionIsIdempotentAndUnique(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	eventID, userOne, userTwo := createPurchaseFixture(t, pool, nil, 1250000, now.Add(2*time.Hour))
	first, err := repo.Create(context.Background(), eventID, userOne, now, time.Minute)
	if err != nil {
		t.Fatalf("create first purchase: %v", err)
	}
	second, err := repo.Create(context.Background(), eventID, userTwo, now, time.Minute)
	if err != nil {
		t.Fatalf("create second purchase: %v", err)
	}
	hash := testdb.UniqueHash("b")
	if _, err := repo.SubmitTransaction(context.Background(), first.ID, userOne, hash, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("submit first purchase: %v", err)
	}
	retry, err := repo.SubmitTransaction(context.Background(), first.ID, userOne, hash, now.Add(time.Second), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("idempotent submit: %v", err)
	}
	if retry.ID != first.ID || retry.TransactionHash == nil || *retry.TransactionHash != hash {
		t.Fatalf("unexpected idempotent retry: %#v", retry)
	}
	if _, err := repo.SubmitTransaction(context.Background(), second.ID, userTwo, hash, now, now.Add(time.Hour)); !errors.Is(err, domainErr.ErrAlreadyExists) {
		t.Fatalf("duplicate hash error = %v, want already exists", err)
	}
}

func TestPurchaseRepositoryExpiredUnresolvedPaymentReleasesCapacity(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	capacity := 1
	eventID, userOne, userTwo := createPurchaseFixture(t, pool, &capacity, 1250000, now.Add(2*time.Hour))
	first, err := repo.Create(context.Background(), eventID, userOne, now, time.Minute)
	if err != nil {
		t.Fatalf("create first purchase: %v", err)
	}
	if _, err := repo.SubmitTransaction(context.Background(), first.ID, userOne, testdb.UniqueHash("c"), now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("submit stale purchase: %v", err)
	}
	if expired, err := repo.ExpireUnresolved(context.Background(), first.ID, userOne, now, time.Hour); err != nil {
		t.Fatalf("expire stale purchase: %v", err)
	} else if expired.Status != model.StatusExpired {
		t.Fatalf("status = %q, want expired", expired.Status)
	}
	replacement, err := repo.Create(context.Background(), eventID, userTwo, now, time.Minute)
	if err != nil {
		t.Fatalf("create replacement after expiry: %v", err)
	}
	if replacement.Status != model.StatusPending {
		t.Fatalf("replacement status = %q, want pending", replacement.Status)
	}
}

func TestPurchaseRepositoryConcurrentDuplicateHashOnlyOneWins(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	eventID, userOne, userTwo := createPurchaseFixture(t, pool, nil, 1250000, now.Add(2*time.Hour))
	first, err := repo.Create(context.Background(), eventID, userOne, now, time.Minute)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := repo.Create(context.Background(), eventID, userTwo, now, time.Minute)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	hash := testdb.UniqueHash("d")
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, item := range []struct {
		id   uuid.UUID
		user uuid.UUID
	}{{first.ID, userOne}, {second.ID, userTwo}} {
		wg.Add(1)
		go func(id, user uuid.UUID) {
			defer wg.Done()
			_, submitErr := repo.SubmitTransaction(context.Background(), id, user, hash, now, now.Add(time.Hour))
			results <- submitErr
		}(item.id, item.user)
	}
	wg.Wait()
	close(results)
	var successes, conflicts int
	for submitErr := range results {
		if submitErr == nil {
			successes++
			continue
		}
		if errors.Is(submitErr, domainErr.ErrAlreadyExists) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent result: %v", submitErr)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestPurchaseRepositoryExpiredRecoveryIsIdempotent(t *testing.T) {
	pool := testPurchaseDB(t)
	repo := NewPurchaseRepo(pool)
	now := time.Now().UTC()
	eventID, userOne, _ := createPurchaseFixture(t, pool, nil, 1250000, now.Add(2*time.Hour))
	purchase, err := repo.Create(context.Background(), eventID, userOne, now, time.Minute)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.SubmitTransaction(context.Background(), purchase.ID, userOne, testdb.UniqueHash("e"), now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	expired, err := repo.ExpireUnresolved(context.Background(), purchase.ID, userOne, now, time.Hour)
	if err != nil || expired.Status != model.StatusExpired {
		t.Fatalf("expire = %#v %v", expired, err)
	}
	confirmed, err := repo.ConfirmExpiredRecovery(context.Background(), purchase.ID, userOne, now)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if confirmed.Status != model.StatusConfirmed {
		t.Fatalf("status = %q", confirmed.Status)
	}
	again, err := repo.ConfirmExpiredRecovery(context.Background(), purchase.ID, userOne, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("idempotent recover: %v", err)
	}
	if again.Status != model.StatusConfirmed {
		t.Fatalf("idempotent status = %q", again.Status)
	}
}
