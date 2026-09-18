package paymentrequest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/nimiqtx"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func testRequestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.Open(t)
}

func createRequestUsers(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	creator, payer := uuid.New(), uuid.New()
	ctx := context.Background()
	for _, userID := range []uuid.UUID{creator, payer} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", userID, userID.String()+"@payment-request-test.local"); err != nil {
			t.Fatalf("insert fixture user: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM consumed_nimiq_transactions
			WHERE domain_type = 'payment_request'
			  AND domain_id IN (
			    SELECT id FROM payment_requests
			    WHERE creator_user_id IN ($1, $2) OR payer_user_id IN ($1, $2)
			  )`, creator, payer)
		_, _ = pool.Exec(ctx, "DELETE FROM payment_requests WHERE creator_user_id IN ($1, $2) OR payer_user_id IN ($1, $2)", creator, payer)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", creator, payer)
	})
	return creator, payer
}

func TestRequestRepositoryConcurrentSubmitAndCrossDomainReplay(t *testing.T) {
	pool := testRequestDB(t)
	repo := NewRequestRepo(pool)
	now := time.Now().UTC()
	creator, payer := createRequestUsers(t, pool)
	first := &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      2500000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	created, err := repo.Create(context.Background(), first)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	hashOne := testdb.UniqueHash("a")
	hashTwo := testdb.UniqueHash("b")
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, hash := range []string{hashOne, hashTwo} {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			_, submitErr := repo.SubmitTransaction(context.Background(), created.PublicID, payer, h, now, now.Add(time.Hour))
			results <- submitErr
		}(hash)
	}
	wg.Wait()
	close(results)
	var successes, conflicts int
	for submitErr := range results {
		if submitErr == nil {
			successes++
			continue
		}
		if errors.Is(submitErr, domainErr.ErrConflict) || errors.Is(submitErr, domainErr.ErrAlreadyExists) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent result: %v", submitErr)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	second := &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      100000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	createdSecond, err := repo.Create(context.Background(), second)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	paid, err := repo.GetByPublicID(context.Background(), created.PublicID, now)
	if err != nil {
		t.Fatalf("reload first: %v", err)
	}
	if paid.TransactionHash == nil {
		t.Fatal("expected first request to have a transaction hash")
	}
	if _, err := repo.SubmitTransaction(context.Background(), createdSecond.PublicID, payer, *paid.TransactionHash, now, now.Add(time.Hour)); !errors.Is(err, domainErr.ErrAlreadyExists) {
		t.Fatalf("intra-domain replay error = %v", err)
	}

	third := &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      200000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	createdThird, err := repo.Create(context.Background(), third)
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	eventHash := testdb.UniqueHash("c")
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := nimiqtx.Consume(context.Background(), tx, eventHash, nimiqtx.DomainEventPurchase, uuid.New(), now); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("consume event purchase hash: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit consumed hash: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM consumed_nimiq_transactions WHERE transaction_hash = $1", eventHash)
	})
	if _, err := repo.SubmitTransaction(context.Background(), createdThird.PublicID, payer, eventHash, now, now.Add(time.Hour)); !errors.Is(err, domainErr.ErrAlreadyExists) {
		t.Fatalf("cross-domain replay error = %v", err)
	}
}

func TestRequestRepositoryConcurrentPayersOnlyOneWins(t *testing.T) {
	pool := testRequestDB(t)
	repo := NewRequestRepo(pool)
	now := time.Now().UTC()
	creator, payerOne := createRequestUsers(t, pool)
	payerTwo := uuid.New()
	if _, err := pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", payerTwo, payerTwo.String()+"@payment-request-test.local"); err != nil {
		t.Fatalf("insert second payer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM consumed_nimiq_transactions
			WHERE domain_type = 'payment_request'
			  AND domain_id IN (SELECT id FROM payment_requests WHERE payer_user_id = $1)`, payerTwo)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_requests WHERE payer_user_id = $1", payerTwo)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", payerTwo)
	})
	created, err := repo.Create(context.Background(), &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      2500000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, payer := range []uuid.UUID{payerOne, payerTwo} {
		wg.Add(1)
		go func(payer uuid.UUID, n int) {
			defer wg.Done()
			hash := testdb.UniqueHash(string(rune('a' + n)))
			_, submitErr := repo.SubmitTransaction(context.Background(), created.PublicID, payer, hash, now, now.Add(time.Hour))
			results <- submitErr
		}(payer, i)
	}
	wg.Wait()
	close(results)
	var successes, conflicts int
	for submitErr := range results {
		if submitErr == nil {
			successes++
			continue
		}
		if errors.Is(submitErr, domainErr.ErrConflict) || errors.Is(submitErr, domainErr.ErrAlreadyExists) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent payer result: %v", submitErr)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("payer successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestRequestRepositoryExpiredRecoveryIsIdempotent(t *testing.T) {
	pool := testRequestDB(t)
	repo := NewRequestRepo(pool)
	now := time.Now().UTC()
	creator, payer := createRequestUsers(t, pool)
	created, err := repo.Create(context.Background(), &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      100000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	hash := testdb.UniqueHash("f")
	if _, err := repo.SubmitTransaction(context.Background(), created.PublicID, payer, hash, now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	expired, err := repo.ExpireUnresolved(context.Background(), created.ID, now, time.Hour)
	if err != nil || expired.Status != model.StatusExpired {
		t.Fatalf("expire = %#v %v", expired, err)
	}
	paid, err := repo.ConfirmExpiredRecovery(context.Background(), created.ID, now)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if paid.Status != model.StatusPaid {
		t.Fatalf("status = %q", paid.Status)
	}
	again, err := repo.ConfirmExpiredRecovery(context.Background(), created.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("idempotent recover: %v", err)
	}
	if again.Status != model.StatusPaid {
		t.Fatalf("idempotent status = %q", again.Status)
	}
}

func TestRequestRepositoryCancelVersusSubmitIsDeterministic(t *testing.T) {
	pool := testRequestDB(t)
	repo := NewRequestRepo(pool)
	now := time.Now().UTC()
	creator, payer := createRequestUsers(t, pool)
	created, err := repo.Create(context.Background(), &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: creator,
		RecipientAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		AmountLunas:      100000, Status: model.StatusPending,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, cancelErr := repo.Cancel(context.Background(), created.PublicID, creator, now)
		results <- cancelErr
	}()
	go func() {
		defer wg.Done()
		_, submitErr := repo.SubmitTransaction(context.Background(), created.PublicID, payer, testdb.UniqueHash("c"), now, now.Add(time.Hour))
		results <- submitErr
	}()
	wg.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, domainErr.ErrConflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected cancel/submit result: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("cancel vs submit successes=%d conflicts=%d", successes, conflicts)
	}
	final, err := repo.GetByPublicID(context.Background(), created.PublicID, now)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if final.Status != model.StatusCancelled && final.Status != model.StatusSubmitted && final.Status != model.StatusVerifying {
		t.Fatalf("final status = %q", final.Status)
	}
}
